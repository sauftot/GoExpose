package main

import (
	"context"
	"crypto/tls"
	"example.com/reverseproxy/pkg/frame"
	"fmt"
	"net"
	"strconv"
	"time"
)

type Proxy struct {
	CtrlConn *tls.Conn
	ip       net.IP
	config   *tls.Config

	ctx            context.Context
	ctxClose       context.CancelFunc
	exposedPorts   map[int]context.CancelFunc
	exposedPortsNr int
}

func NewProxy() *Proxy {
	return &Proxy{
		ip:             nil,
		config:         nil,
		exposedPorts:   make(map[int]context.CancelFunc),
		exposedPortsNr: 0,
	}
}

func (p *Proxy) setConfig(config *tls.Config) {
	p.config = config
}

func (p *Proxy) connectToServer(domainOrIp string, ctx context.Context) {
	ip := net.ParseIP(domainOrIp)
	if ip == nil {
		ip2, err := net.ResolveIPAddr("ip4", domainOrIp)
		if err != nil {
			logger.Error("Error resolving domain name: ", err)
			return
		}
		p.ip = ip2.IP
	} else {
		p.ip = ip
	}
	fmt.Println("[INFO] Connecting to server: ", p.ip.String())
	conn, err := tls.Dial("tcp", p.ip.String()+":"+strconv.Itoa(CTRLPORT), p.config)
	if err != nil {
		logger.Error("Error connecting to server: ", err)
		return
	}
	logger.Log("Connected!")
	tempCtx := context.WithValue(ctx, "port", CTRLPORT)
	p.ctx, p.ctxClose = context.WithCancel(tempCtx)
	// spin off a goroutine to handle the connection
	wg.Add(1)
	p.CtrlConn = conn
	go p.handleServerConnection()
}

func (p *Proxy) handleServerConnection() {
	defer wg.Done()
	defer func() {
		if p.CtrlConn != nil {
			err := p.CtrlConn.Close()
			if err != nil {
				logger.Error("Error closing connection: ", err)
			}
			p.CtrlConn = nil
			p.ctxClose()

		}
	}()
	for {
		select {
		case <-p.ctx.Done():
			return
		default:
			err := p.CtrlConn.SetDeadline(time.Now().Add(1 * time.Second))
			if err != nil {
				logger.Error("Error setting deadline: ", err)
				return
			}
			fr, err := frame.ReadFrame(p.CtrlConn)
			if err != nil {
				logger.Error("Error reading frame from server: ", err)
				return
			}
			logger.Log("Received frame from server: " + strconv.Itoa(int(fr.Typ)))
			switch fr.Typ {
			case frame.CTRLUNPAIR:
				return
			case frame.CTRLCONNECT:
				p.startProxy(fr)
			}
		}

	}
}

func (p *Proxy) expose(ctx context.Context, cancel context.CancelFunc, portStr string) {
	port := ctx.Value("port").(int)
	// send the CTRLEXPOSE with the port to the server
	fr := frame.NewCTRLFrame(frame.CTRLEXPOSETCP, []string{portStr})
	bytes, err := frame.ToByteArray(fr)
	if err != nil {
		fmt.Println("[ERROR] Error creating CTRLFrame!")
		return
	}
	_, err = p.CtrlConn.Write(bytes)
	if err != nil {
		return
	}
	p.exposedPorts[port] = cancel
	p.exposedPortsNr++
}

func (p *Proxy) hide(portStr string) {
	port, err := strconv.Atoi(portStr)
	if err != nil {
		fmt.Println("[ERROR] Invalid port number!")
		return
	}
	if !p.exposedPorts[port] {
		fmt.Println("[ERROR] Port not exposed!")
		return
	}
	// send the CTRLHIDE with the port to the server
	fr := frame.NewCTRLFrame(frame.CTRLHIDETCP, []string{portStr})
	bytes, err := frame.ToByteArray(fr)
	if err != nil {
		fmt.Println("[ERROR] Error creating CTRLFrame!")
		return
	}
	_, err = p.CtrlConn.Write(bytes)
	if err != nil {
		return
	}
	p.exposedPorts[port] = false
	p.exposedPortsNr--
}

func (p *Proxy) startProxy(fr *frame.CTRLFrame) {
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

	pConn, err := net.DialTCP("tcp", nil, &net.TCPAddr{IP: p.ip, Port: pPort})
	if err != nil {
		logger.Error("Error startProxy dialing:", err)
		return
	}

	lConn, err := net.DialTCP("tcp", nil, &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: lPort})
	if err != nil {
		logger.Error("Error startProxy dialing:", err)
		return
	}

	wg.Add(2)
	go p.relayTcp(pConn, lConn, lPort)
	go p.relayTcp(lConn, pConn, lPort)
}

func (p *Proxy) relayTcp(conn1, conn2 *net.TCPConn, port int) {
	defer wg.Done()
	for p.exposedPorts[port] && p.Paired {
		select {
		case <-stop:
			return
		default:
			buf := make([]byte, 1024)
			n, err := conn1.Read(buf)
			if err != nil {
				logger.Error("Error relay reading from external connection:", err)
				return
			}
			_, err = conn2.Write(buf[:n])
			if err != nil {
				logger.Error("Error relay writing to proxy connection:", err)
				return
			}
		}
	}
}
