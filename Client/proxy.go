package main

import (
	"context"
	"crypto/tls"
	"errors"
	in "example.com/reverseproxy/cmd/Utils"
	"fmt"
	"net"
	"strconv"
	"time"
)

type Proxy struct {
	ctx      context.Context
	config   *tls.Config
	ctxClose context.CancelFunc

	exposedPorts   map[int]in.ContextWithCancel
	exposedPortsNr int
	ctrlConn       *tls.Conn
}

func NewProxy(context context.Context, cancel context.CancelFunc, cfg *tls.Config) *Proxy {
	return &Proxy{
		ctx:      context,
		ctxClose: cancel,
		config:   cfg,

		exposedPorts:   make(map[int]in.ContextWithCancel),
		exposedPortsNr: 0,
		ctrlConn:       nil,
	}
}

func (p *Proxy) setConfig(config *tls.Config) {
	p.config = config
}

func (p *Proxy) connectToServer() bool {
	ip := p.ctx.Value("ip").(net.IP)
	logger.Log("Connecting to: " + ip.String() + ":" + CTRLPORT)
	conn, err := tls.Dial("tcp", ip.String()+":"+CTRLPORT, p.config)
	if err != nil {
		logger.Error("Error connecting to server: ", err)
		return false
	}
	logger.Log("Connected!")
	// spin off a goroutine to handle the connection
	wg.Add(1)
	p.ctrlConn = conn
	go p.handleServerConnection()
	return true
}

func (p *Proxy) handleServerConnection() {
	defer wg.Done()
	defer func() {
		if p.ctrlConn != nil {
			err := p.ctrlConn.Close()
			if err != nil {
				logger.Error("Error closing connection in defer: ", err)
			}
			p.ctrlConn = nil
			p.ctxClose()
		}
	}()
	for {
		select {
		case <-p.ctx.Done():
			return
		default:
			err := p.ctrlConn.SetDeadline(time.Now().Add(1 * time.Second))
			if err != nil {
				logger.Error("Error setting deadline: ", err)
				return
			}
			fr, err := in.ReadFrame(p.ctrlConn)
			if err != nil {
				var netErr net.Error
				if errors.As(err, &netErr) && netErr.Timeout() {
					continue
				} else {
					logger.Error("Error reading frame from server: ", err)
					return
				}
			}
			logger.Log("Received frame from server: " + strconv.Itoa(int(fr.Typ)))
			switch fr.Typ {
			case in.CTRLUNPAIR:
				return
			case in.CTRLCONNECT:
				p.startProxy(fr)
			}
		}

	}
}

func (p *Proxy) startProxy(fr *in.CTRLFrame) {
	lPort, err := strconv.Atoi(fr.Data[0])
	if err != nil {
		logger.Error("Error startProxy converting lPort number: ", err)
		return
	}
	pPort, err := strconv.Atoi(fr.Data[1])
	if err != nil {
		logger.Error("Error startProxy converting pPort number: ", err)
		return
	}

	// Dial remote server on proxy port
	pConn, err := net.DialTCP("tcp", nil, &net.TCPAddr{IP: p.ctx.Value("ip").(net.IP), Port: pPort})
	if err != nil {
		logger.Error("Error startProxy dialing remote:", err)
		return
	}

	// Dial local server
	lConn, err := net.DialTCP("tcp", nil, &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: lPort})
	if err != nil {
		logger.Error("Error startProxy dialing local:", err)
		return
	}

	// spin off goroutines with the correct context for the port
	ctx := p.exposedPorts[lPort].Ctx
	wg.Add(2)
	go p.relayTcp(pConn, lConn, ctx)
	go p.relayTcp(lConn, pConn, ctx)
}

func (p *Proxy) relayTcp(conn1, conn2 *net.TCPConn, ctx context.Context) {
	defer wg.Done()
	defer func() {
		err := conn1.Close()
		if err != nil {
			logger.Error("Error relay closing conn1: ", err)
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
				if err.(net.Error).Timeout() {
					continue
				} else {
					logger.Error("Error relay reading from external connection:", err)
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

func (p *Proxy) expose(portStr string) {
	// send the CTRLEXPOSE with the port to the server
	fr := in.NewCTRLFrame(in.CTRLEXPOSETCP, []string{portStr})
	bytes, err := in.ToByteArray(fr)
	if err != nil {
		fmt.Println("[ERROR] Error creating CTRLFrame!")
		return
	}
	_, err = p.ctrlConn.Write(bytes)
	if err != nil {
		return
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		fmt.Println("[ERROR] Invalid port number!")
		return
	}
	ct := context.WithValue(p.ctx, "port", portStr)
	ctx, cancel := context.WithCancel(ct)
	p.exposedPorts[port] = in.ContextWithCancel{Ctx: ctx, Cancel: cancel}
	p.exposedPortsNr++
}

func (p *Proxy) hide(portStr string) {
	port, err := strconv.Atoi(portStr)
	if err != nil {
		fmt.Println("[ERROR] Invalid port number!")
		return
	}
	if p.exposedPorts[port].Ctx == nil {
		fmt.Println("[ERROR] Port not exposed!")
		return
	}
	// send the CTRLHIDE with the port to the server
	fr := in.NewCTRLFrame(in.CTRLHIDETCP, []string{portStr})
	bytes, err := in.ToByteArray(fr)
	if err != nil {
		fmt.Println("[ERROR] Error creating CTRLFrame!")
		return
	}
	_, err = p.ctrlConn.Write(bytes)
	if err != nil {
		return
	}
	p.exposedPorts[port].Cancel()
	p.exposedPorts[port] = in.ContextWithCancel{}
	p.exposedPortsNr--
}
