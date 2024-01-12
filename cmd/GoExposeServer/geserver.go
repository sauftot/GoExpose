package main

import (
	"example.com/reverseproxy/pkg/console"
	"example.com/reverseproxy/pkg/frame"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

type GeServer struct {
	paired     bool
	wg         *sync.WaitGroup
	netOut     chan *frame.CTRLFrame
	proxyPorts map[uint16]bool
	expTCP     map[uint16]bool
	expUDP     map[uint16]bool
}

func newGeServer(wg *sync.WaitGroup) *GeServer {
	return &GeServer{
		paired:     false,
		wg:         wg,
		netOut:     make(chan *frame.CTRLFrame),
		proxyPorts: make(map[uint16]bool, 10),
		expTCP:     make(map[uint16]bool),
		expUDP:     make(map[uint16]bool),
	}
}

func (s *GeServer) run(stop <-chan struct{}) {
	defer s.wg.Done()
	netIn := make(chan *frame.CTRLFrame)

	for {
		select {
		case <-stop:
			s.paired = false
			return
		default:
			s.connectControl(stop, netIn)
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

func (s *GeServer) connectControl(stop <-chan struct{}, netIn chan<- *frame.CTRLFrame) {
	l, err := net.ListenTCP("tcp", &net.TCPAddr{Port: int(frame.CTRLPORT)})
	if err != nil {
		return
	}
	defer l.Close()
	var conn *net.TCPConn

	for !s.paired {
		select {
		case <-stop:
			return
		default:
			fmt.Println("Trying to accept connection...")
			l.SetDeadline(time.Now().Add(1 * time.Second))
			conn, err = l.AcceptTCP()
			if err != nil {
				if opErr := err.(*net.OpError); opErr.Timeout() {
					continue
				} else {
					fmt.Println("ERROR: connectControl: " + err.Error())
					return
				}
			} else {
				conn.Write([]byte("a"))
				conn.SetReadDeadline(time.Now().Add(1 * time.Second))
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
	defer conn.Close()
	defer s.wg.Done()

	s.wg.Add(1)
	go s.netOutHandler(conn)

	for s.paired {
		var buf []byte
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, err := conn.Read(buf)
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
	/*
		TODO: check which proxy port is available, create a listener
		TODO: create a listener on the specified port
		TODO: when an external connection arrives:
			TODO: send the proxy port to the client using CTRLCONNECT over the control channel
			TODO: accept a connection on the proyx port with timeout
			TODO: hand off the two connections to a tcpRelay
	*/
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
	defer lExternal.Close()

	for s.paired && s.expTCP[port] {
		lExternal.SetDeadline(time.Now().Add(500 * time.Millisecond))
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
		lProxy.SetDeadline(time.Now().Add(2 * time.Second))
		cProxy, err := lProxy.AcceptTCP()
		if err != nil {
			panic("ERROR: tcpProxy: " + err.Error())
		} else {
			s.wg.Add(1)
			go s.tcpRelay(cExternal, cProxy, port)
			s.wg.Add(1)
			go s.tcpRelay(cProxy, cExternal, port)
		}
		lProxy.Close()
	}
}

func (s *GeServer) tcpRelay(src, dst *net.TCPConn, port uint16) {
	defer src.Close()
	defer dst.Close()
	defer s.wg.Done()

	buf := make([]byte, 2048)
	for s.paired && s.expTCP[port] {
		src.SetReadDeadline(time.Now().Add(1 * time.Second))
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
