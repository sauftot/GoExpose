package Server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"log/slog"
	"net"
	"os"
	"path/filepath"
)

const (
	CTRLPORT       string = "47921"
	TCPPROXYBASE   int    = 47923
	TCPPROXYAMOUNT int    = 10
)

type Server struct {
	proxy  *Proxy
	Logger *slog.Logger
}

// Run is the main loop of the server. It first initializes the TLS config, then listens for incoming control connections.
// When a connection is accepted, it is handled in a proxy instance until disconnect.
func (s *Server) Run(context context.Context) {
	config := s.prepareTlsConfig()
	if config == nil {
		s.Logger.Error("Error preparing TLS config", slog.String("Func", "Run"))
		return
	}

	for {
		select {
		case <-context.Done():
			return
		default:
			clientConn := s.ctrlListen(context, config)
			if clientConn == nil {
				continue
			}
			s.Logger.Debug("Accepted control connection", slog.String("Address", clientConn.RemoteAddr().String()))
			HandleClient(context, clientConn, s.Logger)
		}
	}
}

// prepareTlsConfig reads the CA certificate, server key and certificate from the user's home directory and creates a tls.Config object.
func (s *Server) prepareTlsConfig() *tls.Config {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		s.Logger.Error("Error getting home directory", slog.String("Func", "prepareTlsConfig"), "Error", err)
		return nil
	}
	filePath := filepath.Join(homeDir, "certs", "myCA.pem")
	caCertData, err := os.ReadFile(filePath)
	if err != nil {
		s.Logger.Error("Error reading CA certificate", slog.String("Func", "prepareTlsConfig"), "Error", err)
		return nil
	}

	caCertPool := x509.NewCertPool()
	ok := caCertPool.AppendCertsFromPEM(caCertData)
	if !ok {
		s.Logger.Error("Error appending CA certificate to pool")
		return nil
	}
	keyPath := filepath.Join(homeDir, "certs", "server.key")
	crtPath := filepath.Join(homeDir, "certs", "server.crt")
	cer, err := tls.LoadX509KeyPair(crtPath, keyPath)
	if err != nil {
		s.Logger.Error("Error loading key pair", slog.String("Func", "prepareTlsConfig"), "Error", err)
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

// ctrlListen starts a TLS listener with the provided config and listens for incoming connections.
// If a connection is accepted, it starts a proxy instance with the connection.
// The function returns the accepted connection and nil if successful, or nil and an error.
//
// TODO: make the error handling more specific, panic in case of hard errors
func (s *Server) ctrlListen(ctx context.Context, config *tls.Config) net.Conn {
	l, err := tls.Listen("tcp", ":"+CTRLPORT, config)
	if err != nil {
		s.Logger.Error("Error TLS listening", slog.String("Func", "ctrlListen"), slog.String("Port", CTRLPORT), "Error", err)
		panic(err)
	}
	// listening context, to close the listener when the main context is cancelled or terminate the helper goroutine when the listener is closed
	lctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// handle a helper goroutine to close the listener when stop is received from console, or when we have accepted a connection
	go func(ctx context.Context, l net.Listener) {
		<-ctx.Done()
		s.Logger.Debug("Closing TLS listener", slog.String("Func", "ctrlListen"))
		err := l.Close()
		if err != nil {
			s.Logger.Debug("Error closing TLS listener", slog.String("Func", "ctrlListen"), "Error", err)
		}
	}(lctx, l)

	conn, err := l.Accept()
	if err != nil {
		s.Logger.Debug("TLS error accepting connection", slog.String("Func", "ctrlListen"), "Error", err)
		return nil
	}

	s.Logger.Debug("Accepted connection, starting proxy", slog.String("Address", conn.RemoteAddr().String()))
	return conn
}
