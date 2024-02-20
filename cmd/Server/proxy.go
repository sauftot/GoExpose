package main

import (
	"context"
	"errors"
	in "example.com/reverseproxy/cmd/internal"
	"net"
	"strconv"
	"time"
)

type Proxy struct {
	PairedIP net.Addr
	NetOut   chan *in.CTRLFrame
	ctx      in.ContextWithCancel

	exposedPorts map[int]in.ContextWithCancel
	proxyPorts   []int
}

func NewProxy(context context.Context, cancel context.CancelFunc) *Proxy {
	return &Proxy{
		PairedIP: context.Value("addr").(net.Addr),
		NetOut:   make(chan *in.CTRLFrame, 100),
		ctx:      in.ContextWithCancel{Ctx: context, Cancel: cancel},

		exposedPorts: make(map[int]in.ContextWithCancel),
		proxyPorts:   make([]int, 0, 10),
	}
}

func (p *Proxy) exposeTcpPreChecks(portCtx in.ContextWithCancel) {
	/*
		Check if the port is already exposed. If not, start an Exposer for the external port.
	*/
	var port = portCtx.Ctx.Value("port").(int)
	if port < 1024 || port > 65535 {
		portCtx.Cancel()
		return
	}
	if len(p.proxyPorts) >= 10 {
		logger.Log("Expose rejected! Max number of ports reached")
		portCtx.Cancel()
		return
	}
	p.proxyPorts = append(p.proxyPorts, port)
	// Start a listener on the port
	logger.Log("Starting exposer for port: " + strconv.Itoa(port))
	wg.Add(1)
	go p.startExposer(portCtx)
}

func (p *Proxy) startExposer(portCtx in.ContextWithCancel) {
	defer wg.Done()
	port := portCtx.Ctx.Value("port").(int)
	defer func() {
		for i, o := range p.proxyPorts {
			if o == port {
				p.proxyPorts = append(p.proxyPorts[:i], p.proxyPorts[i+1:]...)
			}
		}
		p.exposedPorts[port] = in.ContextWithCancel{}
	}()
	// Accept a connection
	// Start a listener on a proxy port
	// Send CTRLCONNECT with the proxy port to the client
	// Wait for the client to connect to the proxy port
	// Hand off the external connection and the connection to the client to relay goroutines
	var proxyPort int
	l, err := net.ListenTCP("tcp", &net.TCPAddr{Port: port})
	if err != nil {
		logger.Error("Error exposer listening:", err)
		return
	}

	for {
		select {
		case <-portCtx.Ctx.Done():
			return
		default:
			err = l.SetDeadline(time.Now().Add(1 * time.Second))
			if err != nil {
				logger.Error("Error exposer setting deadline:", err)
				return
			}
			extConn, err := l.AcceptTCP()
			if err != nil {
				if netErr := err.(net.Error); netErr.Timeout() {
					// healthy timeout
					continue
				} else {
					logger.Error("Error exposer accepting external connection:", err)
					return
				}
			}
			for i := 0; i < len(p.proxyPorts); i++ {
				if p.proxyPorts[i] == port {
					proxyPort = TCPPROXYBASE + i
				}
			}
			// Start a listener on the proxy port
			lProxy, err := net.ListenTCP("tcp", &net.TCPAddr{Port: proxyPort})
			if err != nil {
				logger.Error("Error exposer listening on proxy port:", err)
				return
			}
			p.NetOut <- in.NewCTRLFrame(in.CTRLCONNECT, []string{strconv.Itoa(port),
				strconv.Itoa(proxyPort)})

			// Client has 2 seconds to connect to the proxy port
			err = lProxy.SetDeadline(time.Now().Add(2 * time.Second))
			if err != nil {
				logger.Error("Error exposer setting deadline:", err)
				return
			}
			proxConn, err := lProxy.AcceptTCP()
			if err != nil {
				logger.Error("Error exposer accepting proxy connection:", err)
				return
			}
			ip1, _, _ := net.SplitHostPort(proxConn.RemoteAddr().String())
			ip2, _, _ := net.SplitHostPort(p.PairedIP.String())

			if ip1 != ip2 {
				logger.Error("Error: IP mismatch", errors.New("IP mismatch"))
				return
			}
			// hand off the connections to relayTcp
			logger.Log("Handing off connections to relay goroutines on port: " + strconv.Itoa(port))

			wg.Add(1)
			go p.relayTcp(extConn, proxConn, portCtx.Ctx)
			wg.Add(1)
			go p.relayTcp(proxConn, extConn, portCtx.Ctx)
		}
	}
}

func (p *Proxy) relayTcp(conn1, conn2 *net.TCPConn, ctx context.Context) {
	defer wg.Done()
	defer func() {
		err := conn1.Close()
		if err != nil {
			return
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return
		default:
			err := conn1.SetDeadline(time.Now().Add(1 * time.Second))
			buf := make([]byte, 1024)
			n, err := conn1.Read(buf)
			if err != nil {
				var netErr net.Error
				if errors.As(err, &netErr) && netErr.Timeout() {
					continue
				} else {
					logger.Error("Error relay reading from connection:", err)
					return
				}
			}
			_, err = conn2.Write(buf[:n])
			if err != nil {
				logger.Error("Error relay writing to proxy connection:", err)
				return
			}
		}
	}
}

func (p *Proxy) manageCtrlConnectionOutgoing() {
	defer wg.Done()
	logger.Log("Starting manageCtrlConnectionOutgoing")
	conn := p.ctx.Ctx.Value("conn").(net.Conn)
	for {
		select {
		case <-p.ctx.Ctx.Done():
			return
		case fr := <-p.NetOut:
			if fr.Typ == in.STOP {
				return
			} else {
				err := in.WriteFrame(conn, fr)
				if err != nil {
					logger.Error("Error writing frame:", err)
					return
				}
				if fr.Typ == in.CTRLUNPAIR {
					p.NetOut = make(chan *in.CTRLFrame, 100)
				}
			}
		}
	}
}

func (p *Proxy) manageCtrlConnectionIncoming() {
	conn := p.ctx.Ctx.Value("conn").(net.Conn)
	defer func(conn net.Conn) {
		if conn != nil {
			err := conn.Close()
			if err != nil {

			}
		}
	}(conn)
	logger.Log("Starting manageCtrlConnectionIncoming")

	// Run a helper goroutine to close the connection when stop is received from console
	wg.Add(1)
	go func() {
		wg.Done()
		for {
			select {
			case <-p.ctx.Ctx.Done():
				p.NetOut <- in.NewCTRLFrame(in.CTRLUNPAIR, nil)
				logger.Log("Closing TLS Conn")
				p.NetOut <- in.NewCTRLFrame(in.STOP, nil)
				return
			}
		}
	}()

	for {
		select {
		case <-p.ctx.Ctx.Done():
			return
		default:
			p.handleCtrlFrame()
		}
	}
}

func (p *Proxy) handleCtrlFrame() {
	conn := p.ctx.Ctx.Value("conn").(net.Conn)
	// blocking read!
	fr, err := in.ReadFrame(conn)
	if err != nil {
		logger.Error("Error reading frame, disconnecting:", err)
		p.ctx.Cancel()
		return
	}
	logger.Log("Received frame from ctrlConn: " + strconv.Itoa(int(fr.Typ)) + " " + fr.Data[0])
	switch fr.Typ {
	case in.CTRLUNPAIR:
		p.ctx.Cancel()
	case in.CTRLEXPOSETCP:
		port, err := strconv.Atoi(fr.Data[0])
		if err != nil {
			logger.Error("Error converting port to int:", err)
			return
		}
		ct := context.WithValue(p.ctx.Ctx, "port", port)
		ct2, cancel := context.WithCancel(ct)
		portCtx := in.ContextWithCancel{Ctx: ct2, Cancel: cancel}
		p.exposedPorts[port] = portCtx
		p.exposeTcpPreChecks(portCtx)
	case in.CTRLHIDETCP:
		port, err := strconv.Atoi(fr.Data[0])
		if err != nil {
			logger.Error("Error converting port to int:", err)
			return
		}
		if p.exposedPorts[port].Ctx != nil {
			p.exposedPorts[port].Cancel()
		}
	}
}
