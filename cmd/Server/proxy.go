package Server

import (
	in "Utils"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"strconv"
	"sync"
	"time"
)

/*
	Proxy structs handle one GoExpose client on the server side. Currently, GoExpose is limited to one client per server.

*/

type Proxy struct {
	CtrlConn net.Conn
	NetOut   chan *in.CTRLFrame

	exposedTcpPorts map[int]Relay
	exposedUdpPorts map[int]Relay
	proxyPorts      *Portqueue

	logger *slog.Logger
}

func NewProxy(conn net.Conn, logger *slog.Logger) *Proxy {
	return &Proxy{
		CtrlConn: conn,
		NetOut:   make(chan *in.CTRLFrame, 100),

		exposedTcpPorts: make(map[int]Relay),
		exposedUdpPorts: make(map[int]Relay),
		proxyPorts:      NewPortqueue(),
		logger:          logger,
	}
}

func (p *Proxy) exposeTcpPreChecks(ctx context.Context, externalPort int, wg *sync.WaitGroup) {
	// Parse the port and check if it is within the valid range
	if externalPort < 1024 || externalPort > 65535 {
		return
	}
	// Check if the port is already exposed
	if _, ok := p.exposedTcpPorts[externalPort]; ok {
		return
	}
	// Check if there are any available proxy ports
	proxyPort := p.proxyPorts.GetPort()
	if proxyPort == 0 {
		// No available proxy ports
		return
	}
	p.logger.Debug("Starting exposer", "Port", strconv.Itoa(externalPort))
	portCtx, cnl := context.WithCancel(ctx)
	p.exposedTcpPorts[externalPort] = Relay{proxyPort: proxyPort, cnl: cnl}
	wg.Add(1)
	go p.runExposerForPort(portCtx, externalPort, proxyPort, wg)
}

func (p *Proxy) runExposerForPort(ctx context.Context, externalPort int, proxyPort int, wg *sync.WaitGroup) {
	defer wg.Done()
	l, err := net.ListenTCP("tcp", &net.TCPAddr{Port: externalPort})
	if err != nil {
		p.logger.Error("Error exposer listening", "Error", err)
		return
	}
	defer p.hidePort(externalPort)

	go func(ctx context.Context, l *net.TCPListener) {
		<-ctx.Done()
		err := l.Close()
		if err != nil {
			p.logger.Error("Error exposer closing listener", "Error", err)
		}
	}(ctx, l)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			extConn, err := l.AcceptTCP()
			if err != nil {
				p.logger.Error("Error exposer accepting external connection", "Error", err)
				return
			}
			p.logger.Debug("Accepted external connection", slog.Int("Port", externalPort))
			// Start a listener on the proxy port
			lProxy, err := net.ListenTCP("tcp", &net.TCPAddr{Port: proxyPort})
			if err != nil {
				p.logger.Error("Error exposer listening on proxy port", "Error", err)
				return
			}
			p.NetOut <- in.NewCTRLFrame(in.CTRLCONNECT, []string{strconv.Itoa(externalPort),
				strconv.Itoa(proxyPort)})

			// Client has 2 seconds to connect to the proxy port
			err = lProxy.SetDeadline(time.Now().Add(2 * time.Second))
			if err != nil {
				p.logger.Error("Error exposer setting deadline:", err)
				return
			}
			proxConn, err := lProxy.AcceptTCP()
			if err != nil {
				p.logger.Error("Error exposer accepting proxy connection:", err)
				return
			}

			// Check if the IPs match with CtrlConn
			ip1, _, _ := net.SplitHostPort(proxConn.RemoteAddr().String())
			ip2, _, _ := net.SplitHostPort(p.CtrlConn.RemoteAddr().String())

			if ip1 != ip2 {
				p.logger.Error("Error: IP mismatch", "IP1", ip1, "IP2", ip2)
				return
			}
			// hand off the connections to RelayTcp
			p.logger.Debug("Handing off connections to relay goroutines", "Port", strconv.Itoa(externalPort))

			wg.Add(1)
			go p.RelayTcp(extConn, proxConn, ctx, wg)
			wg.Add(1)
			go p.RelayTcp(proxConn, extConn, ctx, wg)
		}
	}
}

func (p *Proxy) RelayTcp(conn1, conn2 *net.TCPConn, ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	defer func() {
		_ = conn1.Close()
		_ = conn2.Close()
	}()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			_, err := io.Copy(conn2, conn1)
			if err != nil {
				if errors.As(err, &net.ErrClosed) {
					return
				}
				p.logger.Error("Error relay copying", "Error", err)
				return
			}
		}
	}
}

func (p *Proxy) manageCtrlConnectionOutgoing(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	p.logger.Debug("Starting manageCtrlConnectionOutgoing")
	for {
		select {
		case <-ctx.Done():
			return
		case fr := <-p.NetOut:
			if fr.Typ == in.STOP {
				return
			} else {
				err := in.WriteFrame(p.CtrlConn, fr)
				if err != nil {
					p.logger.Error("Error writing frame:", err)
					return
				}
				if fr.Typ == in.CTRLUNPAIR {
					p.NetOut = make(chan *in.CTRLFrame, 100)
				}
			}
		}
	}
}

func (p *Proxy) manageCtrlConnectionIncoming(ctx context.Context, wg *sync.WaitGroup) {
	p.logger.Debug("Starting manageCtrlConnectionIncoming")
	// this context synchronizes all proxies to the connection of the CtrlConn. If it terminates, all proxies will be closed.
	connCtx, cancel := context.WithCancel(ctx)
	// suppressing warning, if the parent context is cancelled everything should be fine but the warning is annoying
	defer cancel()

	// Run a helper goroutine to close the connection when stop is received from console
	wg.Add(1)
	go func(conn net.Conn) {
		defer wg.Done()
		p.logger.Debug("mCCI subroutine: Waiting for ctx to be done")
		<-connCtx.Done()
		p.NetOut <- in.NewCTRLFrame(in.CTRLUNPAIR, nil)
		p.logger.Debug("Closing TLS CtrlConn")
		p.NetOut <- in.NewCTRLFrame(in.STOP, nil)
		err := conn.Close()
		if err != nil {
			p.logger.Error("Error closing TLS CtrlConn", "Error", err)
		}
		p.logger.Debug("mCCI subroutine: Ctx done")
		return
	}(p.CtrlConn)

	for {
		select {
		case <-connCtx.Done():
			return
		default:
			p.handleCtrlFrame(connCtx, cancel, wg)
		}
	}
}

func (p *Proxy) handleCtrlFrame(ctx context.Context, cancel context.CancelFunc, wg *sync.WaitGroup) {
	// blocking read!
	fr, err := in.ReadFrame(p.CtrlConn)
	if err != nil {
		p.logger.Error("Error reading frame, disconnecting", "Error", err)
		cancel()
		return
	}
	p.logger.Debug("Received frame from ctrlConn: " + strconv.Itoa(int(fr.Typ)) + " " + fr.Data[0])
	switch fr.Typ {
	case in.CTRLUNPAIR:
		p.logger.Info("Received unpair command")
		cancel()
	case in.CTRLEXPOSETCP:
		p.logger.Info("Received exposetcp command", slog.String("port", fr.Data[0]))
		port, err := strconv.Atoi(fr.Data[0])
		if err != nil {
			p.logger.Error("Error converting port to int", "Error", err)
			return
		}
		p.exposeTcpPreChecks(ctx, port, wg)
	case in.CTRLHIDETCP:
		p.logger.Info("Received hidetcp command", slog.String("port", fr.Data[0]))
		port, err := strconv.Atoi(fr.Data[0])
		if err != nil {
			p.logger.Error("Error converting port to int", "Error", err)
			return
		}
		p.hidePort(port)
	case in.CTRLEXPOSEUDP:
		p.logger.Info("Received exposeudp command", slog.String("port", fr.Data[0]))
	case in.CTRLHIDEUDP:
		p.logger.Info("Received hideudp command", slog.String("port", fr.Data[0]))
	}
}

func (p *Proxy) hidePort(port int) {
	if relay, ok := p.exposedTcpPorts[port]; ok {
		relay.cancel()
		p.proxyPorts.ReturnPort(relay.proxyPort)
	}
	delete(p.exposedTcpPorts, port)
}
