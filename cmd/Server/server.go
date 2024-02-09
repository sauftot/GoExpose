package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	in "example.com/reverseproxy/cmd/internal"
	"net"
	"os"
	"path/filepath"
	"strconv"
)

const (
	CTRLPORT     int = 47921
	TCPPROXYBASE int = 47923
)

type Server struct {
	proxy  *Proxy
	config *tls.Config

	ctx      context.Context
	exposers map[int]context.CancelFunc
}

func NewServer(context context.Context) *Server {
	return &Server{
		proxy: NewState(),
		ctx:   context,
	}
}

func (s *Server) run() {
	defer wg.Done()
	s.config = s.prepareTlsConfig()
	if s.config == nil {
		logger.Error("Error preparing TLS config:", nil)
		return
	}

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			s.proxy.CleanUp()
			logger.Log("Waiting for client to connect...")
			conn := s.waitForCtrlConnection()
			if conn != nil {
				logger.Log("Client connected: " + conn.RemoteAddr().String())
				// Run a goroutine that will handle all writes to the ctrl connection
				wg.Add(1)
				go s.manageCtrlConnectionOutgoing(conn)
				// Keep reading from the ctrl connection till disconnected or closed
				s.manageCtrlConnectionIncoming(conn)
			}
		}
	}
}

func (s *Server) prepareTlsConfig() *tls.Config {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		logger.Error("Error getting home directory:", err)
		return nil
	}
	filePath := filepath.Join(homeDir, "certs", "myCA.pem")
	caCertData, err := os.ReadFile(filePath)
	if err != nil {
		logger.Error("Error reading CA certificate:", err)
		return nil
	}

	caCertPool := x509.NewCertPool()
	ok := caCertPool.AppendCertsFromPEM(caCertData)
	if !ok {
		logger.Error("Error appending CA certificate to pool.", nil)
		return nil
	}
	keyPath := filepath.Join(homeDir, "certs", "server.key")
	crtPath := filepath.Join(homeDir, "certs", "server.crt")
	cer, err := tls.LoadX509KeyPair(crtPath, keyPath)
	if err != nil {
		logger.Error("Error loading key pair:", err)
		return nil
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cer},
		ClientCAs:    caCertPool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
	}
	return tlsConfig
}

func (s *Server) waitForCtrlConnection() net.Conn {
	l, err := tls.Listen("tcp", ":"+strconv.Itoa(CTRLPORT), s.config)
	if err != nil {
		logger.Error("Error TLS listening:", err)
		return nil
	}
	stopCauseAccept := make(chan struct{})
	defer close(stopCauseAccept)
	defer func(l net.Listener) {
		if l != nil {
			logger.Log("DEFER closing listener")
			err = l.Close()
			if err != nil {
				logger.Error("Error closing listener:", err)
			}
		}
	}(l)

	// Run a helper goroutine to close the listener when stop is received from console
	wg.Add(1)
	go func(ctx context.Context, stopBecauseAccept chan struct{}, l net.Listener) {
		defer wg.Done()
		for dontClose := true; dontClose; {
			select {
			case <-ctx.Done():
				dontClose = false
				logger.Log("Closing TLS listener")
				err := l.Close()
				if err != nil {
					logger.Error("Error closing TLS listener:", err)
				}
				l = nil
			case <-stopBecauseAccept:
				dontClose = false
			}
		}
		return
	}(s.ctx, stopCauseAccept, l)

	conn, err := l.Accept()
	if err != nil {
		logger.Error("Error accepting connection:", err)
		return nil
	}
	s.proxy.PairedIP = conn.RemoteAddr()
	s.proxy.Paired = true
	return conn
}

func (s *Server) manageCtrlConnectionOutgoing(conn net.Conn) {
	defer wg.Done()
	logger.Log("Starting manageCtrlConnectionOutgoing")
	s.proxy.NetOut = make(chan *in.CTRLFrame, 100)
	for {
		select {
		case <-s.ctx.Done():
			return
		case fr := <-s.proxy.NetOut:
			if fr.Typ == in.STOP {
				return
			} else {
				err := in.WriteFrame(conn, fr)
				if err != nil {
					logger.Error("Error writing frame:", err)
					return
				}
				if fr.Typ == in.CTRLUNPAIR {
					s.proxy.NetOut = make(chan *in.CTRLFrame, 100)
				}
			}
		}
	}
}

func (s *Server) manageCtrlConnectionIncoming(conn net.Conn) {
	defer func(conn net.Conn) {
		if conn != nil {
			err := conn.Close()
			if err != nil {

			}
		}
	}(conn)
	stopCauseConnDead := make(chan struct{})
	defer close(stopCauseConnDead)
	logger.Log("Starting manageCtrlConnectionIncoming")

	// Run a helper goroutine to close the connection when stop is received from console
	wg.Add(1)
	go func(ctx context.Context, stopCauseConnDead chan struct{}) {
		wg.Done()
		for dontClose := true; dontClose; {
			select {
			case <-ctx.Done():
				dontClose = false
				s.proxy.Paired = false
				s.proxy.NetOut <- in.NewCTRLFrame(in.CTRLUNPAIR, nil)
				logger.Log("Closing TLS Conn")
				s.proxy.NetOut <- in.NewCTRLFrame(in.STOP, nil)
			case <-stopCauseConnDead:
				dontClose = false
			}
		}
		return
	}(s.ctx, stopCauseConnDead)

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			if s.proxy.Paired {
				s.handleCtrlFrame(conn)
			} else {
				logger.Log("IncomingHandler returning due to disconnect")
				return
			}
		}
	}
}

func (s *Server) handleCtrlFrame(conn net.Conn) {
	fr, err := in.ReadFrame(conn)
	if err != nil {
		logger.Error("Error reading frame, disconnecting:", err)
		s.proxy.Paired = false
		return
	}
	logger.Log("Received frame from ctrlConn: " + strconv.Itoa(int(fr.Typ)) + " " + fr.Data[0])
	switch fr.Typ {
	case in.CTRLUNPAIR:
		s.proxy.Paired = false
	case in.CTRLEXPOSETCP:
		port, err := strconv.Atoi(fr.Data[0])
		if err != nil {
			logger.Error("Error converting port to int:", err)
			return
		}
		c := context.WithValue(s.ctx, "port", port)
		tcpContext, cancel := context.WithCancel(c)
		s.exposers[port] = cancel
		s.proxy.ExposeTcp(tcpContext)
	case in.CTRLHIDETCP:
		port, err := strconv.Atoi(fr.Data[0])
		if err != nil {
			logger.Error("Error converting port to int:", err)
			return
		}
		if s.exposers[port] != nil {
			s.exposers[port]()
			delete(s.exposers, port)
		}
	}
}
