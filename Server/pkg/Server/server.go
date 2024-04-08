package Server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sync"
)

const (
	CTRLPORT       string = "47921"
	TCPPROXYBASE   int    = 47923
	TCPPROXYAMOUNT int    = 10
)

type Server struct {
	proxy  *Proxy
	logger *slog.Logger
}

func (s *Server) run(context context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	config := s.prepareTlsConfig()
	if config == nil {
		s.logger.Error("Error preparing TLS config:", nil)
		return
	}

	for {
		select {
		case <-context.Done():
			return
		default:
			s.logger.Info("Waiting for client to connect", slog.String("Port", CTRLPORT))
			s.waitForCtrlConnection(context, config)
			s.logger.Info("Client connected", slog.String("IP", s.proxy.CtrlConn.RemoteAddr().String()))
			// Run a goroutine that will handle all writes to the ctrl connection
			wg.Add(1)
			go s.proxy.manageCtrlConnectionOutgoing(context, wg)
			// Keep reading from the ctrl connection till disconnected or closed
			s.proxy.manageCtrlConnectionIncoming(context, wg)
			s.logger.Info("Client disconnected", slog.String("IP", s.proxy.CtrlConn.RemoteAddr().String()))
			// clean up
		}
	}
}

func (s *Server) prepareTlsConfig() *tls.Config {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		s.logger.Error("Error getting home directory:", err)
		return nil
	}
	filePath := filepath.Join(homeDir, "certs", "myCA.pem")
	caCertData, err := os.ReadFile(filePath)
	if err != nil {
		s.logger.Error("Error reading CA certificate:", err)
		return nil
	}

	caCertPool := x509.NewCertPool()
	ok := caCertPool.AppendCertsFromPEM(caCertData)
	if !ok {
		s.logger.Error("Error appending CA certificate to pool.", nil)
		return nil
	}
	keyPath := filepath.Join(homeDir, "certs", "server.key")
	crtPath := filepath.Join(homeDir, "certs", "server.crt")
	cer, err := tls.LoadX509KeyPair(crtPath, keyPath)
	if err != nil {
		s.logger.Error("Error loading key pair:", err)
		return nil
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cer},
		ClientCAs:    caCertPool,
		// The main purpose of this is to verify the client certificate
		ClientAuth: tls.RequireAndVerifyClientCert,
	}
	return tlsConfig
}

func (s *Server) waitForCtrlConnection(ctx context.Context, config *tls.Config) {
	l, err := tls.Listen("tcp", ":"+CTRLPORT, config)
	if err != nil {
		s.logger.Error("Error TLS listening", slog.String("Port", CTRLPORT), "Error", err)
		panic(err)
	}
	listeningCtx, listCancel := context.WithCancel(ctx)
	defer listCancel()

	// Run a helper goroutine to close the listener when stop is received from console
	go func(ctx context.Context, l net.Listener) {
		s.logger.Debug("Starting TLS listener")
		<-ctx.Done()
		s.logger.Debug("Closing TLS listener")
		err := l.Close()
		if err != nil {
			s.logger.Debug("Error closing TLS listener:", err)
		}
		l = nil
		s.logger.Debug("Stopping TLS listener")
	}(listeningCtx, l)

	conn, err := l.Accept()
	if err != nil {
		s.logger.Debug("Error accepting connection:", err)
		return
	}

	s.logger.Debug("Accepted connection, starting proxy", slog.String("Address", conn.RemoteAddr().String()))

	s.proxy = NewProxy(conn, s.logger)
	return
}
