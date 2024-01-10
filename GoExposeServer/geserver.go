package main

import (
	"example.com/reverseproxy/pkg/console"
	"example.com/reverseproxy/pkg/frame"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

type GeServer struct {
	paired bool
	wg     *sync.WaitGroup
	netOut chan *frame.CTRLFrame
	expTCP map[uint16]bool
	expUDP map[uint16]bool
}

func newGeServer(wg *sync.WaitGroup) *GeServer {
	return &GeServer{
		paired: false,
		wg:     wg,
		expTCP: make(map[uint16]bool),
		expUDP: make(map[uint16]bool),
	}
}

func (s *GeServer) run(stop <-chan bool) {
	netIn := make(chan *frame.CTRLFrame)
	s.netOut = make(chan *frame.CTRLFrame)

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

func (s *GeServer) connectControl(stop <-chan bool, netIn chan<- *frame.CTRLFrame) {
	l, err := net.ListenTCP("tcp", &net.TCPAddr{Port: int(frame.CTRLPORT)})
	if err != nil {
		return
	}
	defer l.Close()
	var conn *net.TCPConn = nil
	for conn != nil {
		select {
		case <-stop:
			return
		default:
			l.SetDeadline(time.Now().Add(1 * time.Second))
			conn, err = l.AcceptTCP()
			if opErr := err.(*net.OpError); opErr.Timeout() {
				continue
			} else if err != nil {
				fmt.Println("ERROR: connectControl: " + err.Error())
				return
			} else {
				conn.Write([]byte("a"))
				conn.SetReadDeadline(time.Now().Add(1 * time.Second))
				var buf []byte
				_, err = conn.Read(buf)
				if err != nil && strings.Compare(string(buf), frame.TOKEN) == 0 {
					conn.Close()
					conn = nil
				}
			}
		}
	}
	s.paired = true
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
		if netErr := err.(net.Error); netErr.Timeout() {
			// healthy timeout
			continue
		} else if err != nil {
			fmt.Println("ERROR: controlHandler: " + err.Error())
			fmt.Println("Unpairing...")
			s.paired = false
			return
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
			// TODO: check if a proxy port is available
			// TODO: hand off the proxy port and the port to expose to a new tcp proxy

		}
	case frame.CTRLHIDETCP:
		// TODO: implement
	case frame.CTRLEXPOSEUDP:
		// TODO: implement
	case frame.CTRLHIDEUDP:
		// TODO: implement
	}
}
