package main

import (
	"crypto/tls"
	"crypto/x509"
	main2 "example.com/reverseproxy/cmd/GoExposeServer"
	"example.com/reverseproxy/pkg/console"
	"example.com/reverseproxy/pkg/frame"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type GeServer struct {
	paired     bool
	netOut     chan *frame.CTRLFrame
	proxyPorts map[uint16]bool
	expTCP     map[uint16]bool
	expUDP     map[uint16]bool
}

func newGeServer(wg *sync.WaitGroup, logger *slog.Logger) *GeServer {
	return &GeServer{
		paired:     false,
		netOut:     make(chan *frame.CTRLFrame),
		proxyPorts: make(map[uint16]bool, 10),
		expTCP:     make(map[uint16]bool),
		expUDP:     make(map[uint16]bool),
	}
}

func (s *GeServer) run(stop <-chan struct{}) {
	defer s.wg.Done()

	config := s.prepareTlsConfig()
	if config == nil {
		s.logger.Error("Error preparing TLS config.")
		return
	}

	netIn := make(chan *frame.CTRLFrame)

	for {
		select {
		case <-stop:
			s.paired = false
			return
		default:
			s.connectControl(stop, netIn, config)
			for s.paired {
				select {
				case <-stop:
					s.paired = false
					return
				case fr := <-netIn:
					s.handleControlFrame(fr)
				}
			}
		}
	}
}

func (s *GeServer) connectControl(stop <-chan struct{}, netIn chan<- *frame.CTRLFrame, config *tls.Config) {

	l, err := tls.Listen("tcp", ":"+strconv.Itoa(int(main2.CTRLPORT)), config)
	if err != nil {
		return
	}
	l := la.(*net.TCPListener)
	defer func(l *net.TCPListener) {
		err := l.Close()
		if err != nil {

		}
	}(l)
	var conn *net.TCPConn

	for !s.paired {
		select {
		case <-stop:
			return
		default:
			fmt.Println("Trying to accept connection...")
			err = l.SetDeadline(time.Now().Add(1 * time.Second))
			if err != nil {
				return
			}
			conn, err = l.AcceptTCP()
			if err != nil {
				if opErr := err.(*net.OpError); opErr.Timeout() {
					continue
				} else {
					fmt.Println("ERROR: connectControl: " + err.Error())
					return
				}
			} else {
				_, err := conn.Write([]byte("a"))
				if err != nil {
					return
				}
				err = conn.SetReadDeadline(time.Now().Add(1 * time.Second))
				if err != nil {
					return
				}
				var buf []byte
				_, err = conn.Read(buf)
				if err == nil && strings.Compare(string(buf), frame.TOKEN) == 0 {
					s.paired = true
				}
			}
		}
	}
	fmt.Println("Paired!")
	s.wg.Add(1)
	go s.controlHandler(conn, netIn)
	return
}

func (s *GeServer) controlHandler(conn *net.TCPConn, netIn chan<- *frame.CTRLFrame) {
	defer func(conn *net.TCPConn) {
		err := conn.Close()
		if err != nil {

		}
	}(conn)
	defer s.wg.Done()

	s.wg.Add(1)
	go s.netOutHandler(conn)

	for s.paired {
		var buf []byte
		err := conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		if err != nil {
			return
		}
		_, err = conn.Read(buf)
		if err != nil {
			if netErr := err.(net.Error); netErr.Timeout() {
				// healthy timeout
				continue
			} else {
				fmt.Println("ERROR: controlHandler: " + err.Error())
				fmt.Println("Unpairing...")
				s.paired = false
				return
			}
		} else {
			fr, err := frame.FromByteArray(buf)
			if err != nil {
				fmt.Println("ERROR: controlHandler: " + err.Error())
				fmt.Println("Unpairing...")
				s.paired = false
				return
			} else {
				if fr.Typ == frame.CTRLUNPAIR {
					s.paired = false
					for i, proxy := range s.proxyPorts {
						if proxy {
							s.proxyPorts[i] = false
						}
					}
					return
				} else {
					netIn <- fr
				}
			}
		}
	}
}

func (s *GeServer) netOutHandler(conn *net.TCPConn) {
	s.wg.Done()
	for s.paired {
		select {
		case fr := <-s.netOut:
			fmt.Println("Sending frame to client...")
			jsonBytes, err := frame.ToByteArray(fr)
			if err != nil {
				return
			}
			_, err = conn.Write(jsonBytes)
			if err != nil {
				fmt.Println("ERROR: netOutHandler: " + err.Error())
				fmt.Println("Unpairing...")
				s.paired = false
				return
			}
		}
	}
}

func (s *GeServer) handleControlFrame(fr *frame.CTRLFrame) {
	switch fr.Typ {
	case frame.CTRLEXPOSETCP:
		port, err := console.CheckPort(fr.Data[0])
		if err != nil {
			return
		}
		if !s.expTCP[port] {
			s.expTCP[port] = true
			s.wg.Add(1)
			go s.tcpProxy(port)
		}
	case frame.CTRLHIDETCP:
		port, err := console.CheckPort(fr.Data[0])
		if err != nil {
			return
		}
		if s.expTCP[port] {
			s.expTCP[port] = false
		}
	case frame.CTRLEXPOSEUDP:
		// TODO: implement
	case frame.CTRLHIDEUDP:
		// TODO: implement
	}
}

func (s *GeServer) tcpProxy(port uint16) {
	var proxyPort uint16 = 65535
	for i, proxy := range s.proxyPorts {
		if !proxy {
			s.proxyPorts[i] = true
			proxyPort = i
		}
	}
	if proxyPort == 65535 {
		fmt.Println("ERROR: tcpProxy: no proxy ports available, telling client")
		s.netOut <- &frame.CTRLFrame{
			Typ:  frame.CTRLHIDETCP,
			Data: []string{strconv.Itoa(int(port))},
		}
		return
	}
	proxyPort = frame.TCPPROXYBASE + proxyPort

	lExternal, err := net.ListenTCP("tcp", &net.TCPAddr{Port: int(port)})
	if err != nil {
		panic("ERROR: tcpProxy: " + err.Error())
	}
	defer func(lExternal *net.TCPListener) {
		err := lExternal.Close()
		if err != nil {

		}
	}(lExternal)

	for s.paired && s.expTCP[port] {
		err = lExternal.SetDeadline(time.Now().Add(500 * time.Millisecond))
		if err != nil {
			return
		}
		cExternal, err := lExternal.AcceptTCP()
		if err != nil {
			if netErr := err.(net.Error); netErr.Timeout() {
				continue
			} else if err != nil {
				panic("ERROR: tcpProxy: " + err.Error())
			}
		}

		lProxy, err := net.ListenTCP("tcp", &net.TCPAddr{Port: int(proxyPort)})
		if err != nil {
			panic("ERROR: tcpProxy: " + err.Error())
		}

		s.netOut <- &frame.CTRLFrame{Typ: frame.CTRLCONNECT, Data: []string{strconv.Itoa(int(proxyPort)), strconv.Itoa(int(port))}}
		err = lProxy.SetDeadline(time.Now().Add(2 * time.Second))
		if err != nil {
			return
		}
		cProxy, err := lProxy.AcceptTCP()
		if err != nil {
			panic("ERROR: tcpProxy: " + err.Error())
		} else {
			s.wg.Add(1)
			go s.tcpRelay(cExternal, cProxy, port)
			s.wg.Add(1)
			go s.tcpRelay(cProxy, cExternal, port)
		}
		err = lProxy.Close()
		if err != nil {
			return
		}
	}
}

func (s *GeServer) tcpRelay(src, dst *net.TCPConn, port uint16) {
	defer func(src *net.TCPConn) {
		err := src.Close()
		if err != nil {

		}
	}(src)
	defer func(dst *net.TCPConn) {
		err := dst.Close()
		if err != nil {

		}
	}(dst)
	defer s.wg.Done()

	buf := make([]byte, 2048)
	for s.paired && s.expTCP[port] {
		err := src.SetReadDeadline(time.Now().Add(1 * time.Second))
		if err != nil {
			return
		}
		n, err := src.Read(buf)
		if err != nil {
			if netErr := err.(net.Error); netErr.Timeout() {
				continue
			} else {
				fmt.Println("ERROR: tcpRelay: " + err.Error())
				return
			}
		} else {
			_, err = dst.Write(buf[:n])
			if err != nil {
				fmt.Println("ERROR: tcpRelay: " + err.Error())
				return
			}
			buf = []byte{}
		}
	}
}

func (s *GeServer) prepareTlsConfig() *tls.Config {
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
		s.logger.Error("Error appending CA certificate to pool.")
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
		ClientAuth:   tls.RequireAndVerifyClientCert,
	}
	return tlsConfig
}
