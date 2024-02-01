package main

import (
	"crypto/tls"
	"crypto/x509"
	"example.com/reverseproxy/pkg/frame"
	"net"
	"os"
	"path/filepath"
	"strconv"
)

const (
	CTRLPORT     int = 47921
	UDPPROXYPORT int = 47922
	TCPPROXYBASE int = 47923
	NRTCPPORTS   int = 10
)

type Server struct {
	state  *State
	config *tls.Config
}

func NewServer() *Server {
	return &Server{
		state: NewState(),
	}
}

func (s *Server) run() {
	defer wg.Done()
	s.config = s.prepareTlsConfig()

	for {
		select {
		case <-stop:
			return
		default:
			conn := s.waitForCtrlConnection()
			if conn != nil {
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
	defer func(l net.Listener) {
		logger.Log("Closing TLS listener")
		err := (l).Close()
		if err != nil {
			return
		}
		return
	}(l)
	stopCauseAccept := make(chan struct{})
	defer close(stopCauseAccept)

	// Run a helper goroutine to close the listener when stop is received from console
	wg.Add(1)
	go func(stop chan struct{}, stopBecauseAccept chan struct{}, l net.Listener) {
		defer wg.Done()
		for dontClose := true; dontClose; {
			select {
			case <-stop:
				dontClose = false
				logger.Log("Closing TLS listener")
				err := l.Close()
				if err != nil {
					logger.Error("Error closing TLS listener:", err)
				}
			case <-stopBecauseAccept:
				dontClose = false
			}
		}
		return
	}(stop, stopCauseAccept, l)

	conn, err := l.Accept()
	if err != nil {
		logger.Error("Error accepting connection:", err)
		return nil
	}
	logger.Log("Client connected: " + conn.RemoteAddr().String())
	return conn
}

func (s *Server) manageCtrlConnectionOutgoing(conn net.Conn) {
	defer wg.Done()

	for {
		select {
		case <-stop:
			return
		case fr := <-s.state.NetOut:
			if fr.Typ == frame.STOP {
				return
			} else {
				err := frame.WriteFrame(conn, fr)
				if err != nil {
					logger.Error("Error writing frame:", err)
					return
				}
			}
		}
	}
}

func (s *Server) manageCtrlConnectionIncoming(conn net.Conn) {
	defer func(conn net.Conn) {
		err := conn.Close()
		if err != nil {

		}
	}(conn)
	stopCauseConnDead := make(chan struct{})
	defer close(stopCauseConnDead)

	// Run a helper goroutine to close the connection when stop is received from console
	wg.Add(1)
	go func(stop chan struct{}, stopCauseConnDead chan struct{}) {
		wg.Done()
		for dontClose := true; dontClose; {
			select {
			case <-stop:
				dontClose = false
				s.state.NetOut <- frame.NewCTRLFrame(frame.CTRLUNPAIR, nil)
				logger.Log("Closing TLS Conn")
				s.state.NetOut <- frame.NewCTRLFrame(frame.STOP, nil)
			case <-stopCauseConnDead:
				dontClose = false
			}
		}
		return
	}(stop, stopCauseConnDead)

	for {
		select {
		case <-stop:
			return
		default:
			if s.state.Paired {
				s.handleCtrlFrame(conn)
			} else {
				return
			}
		}
	}
}

func (s *Server) handleCtrlFrame(conn net.Conn) {
	fr, err := frame.ReadFrame(conn)
	if err != nil {
		logger.Error("Error reading frame, disconnecting:", err)
		s.state.Paired = false
		return
	}
	switch fr.Typ {
	case frame.CTRLUNPAIR:
		s.state.Paired = false
	case frame.CTRLEXPOSETCP:
		port, err := strconv.Atoi(fr.Data[0])
		if err != nil {
			logger.Error("Error converting port to int:", err)
			return
		}
		s.state.ExposeTcp(port)
	case frame.CTRLHIDETCP:
		port, err := strconv.Atoi(fr.Data[0])
		if err != nil {
			logger.Error("Error converting port to int:", err)
			return
		}
		s.state.HideTcp(port)
	}
}
