package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"os"
	"path/filepath"
)

const (
	CTRLPORT     string = "47921"
	TCPPROXYBASE int    = 47923
)

type Server struct {
	proxy  *Proxy
	config *tls.Config

	ctx context.Context
}

func NewServer(context context.Context) *Server {
	return &Server{
		proxy: nil,
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
			logger.Log("Waiting for client to connect...")
			s.waitForCtrlConnection()
			if s.proxy != nil {
				logger.Log("Client connected: " + s.proxy.PairedIP.String())
				// Run a goroutine that will handle all writes to the ctrl connection
				wg.Add(1)
				go s.proxy.manageCtrlConnectionOutgoing()
				// Keep reading from the ctrl connection till disconnected or closed
				s.proxy.manageCtrlConnectionIncoming()
				// clean up
				s.proxy = nil

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

func (s *Server) waitForCtrlConnection() {
	l, err := tls.Listen("tcp", ":"+CTRLPORT, s.config)
	if err != nil {
		logger.Error("Error TLS listening:", err)
		panic(err)
	}
	listeningCtx, listCancel := context.WithCancel(s.ctx)
	defer listCancel()

	// Run a helper goroutine to close the listener when stop is received from console
	go func(ctx context.Context, l net.Listener) {
		logger.Debug("Starting TLS listener")
		<-ctx.Done()
		logger.Log("Closing TLS listener")
		err := l.Close()
		if err != nil {
			logger.Error("Error closing TLS listener:", err)
		}
		l = nil
		logger.Debug("Stopping TLS listener")
	}(listeningCtx, l)

	conn, err := l.Accept()
	if err != nil {
		logger.Error("Error accepting connection:", err)
		return
	}

	nt := context.WithValue(s.ctx, "addr", conn.RemoteAddr())
	newCt := context.WithValue(nt, "conn", conn)
	proxCtx, cancel := context.WithCancel(newCt)
	s.proxy = NewProxy(proxCtx, cancel)
	return
}
