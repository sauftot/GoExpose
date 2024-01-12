package main

import (
	"bufio"
	"errors"
	"example.com/reverseproxy/pkg/frame"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"
)

type Networker struct {
	wg       *sync.WaitGroup
	paired   bool
	ctrlConn *net.TCPConn
	expTCP   map[uint16]bool
	expUDP   map[uint16]bool
}

func newNetworker(wg *sync.WaitGroup) *Networker {
	return &Networker{
		wg:       wg,
		paired:   false,
		ctrlConn: nil,
		expTCP:   make(map[uint16]bool),
		expUDP:   make(map[uint16]bool),
	}
}

func (n *Networker) manageCtrl() {
	defer n.wg.Done()
	defer n.ctrlConn.Close()

	r := bufio.NewReader(n.ctrlConn)
	var buf []byte
	for n.paired {
		// check if control connection has new data
		n.ctrlConn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		_, err := r.Read(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				// healthy timeout, continue
				continue
			} else {
				fmt.Println("CONTROLMANAGER: Non-timeout error reading from control connection:", err.Error())
				n.paired = false
				return
			}
		} else {
			fr, err := frame.FromByteArray(buf)
			if err != nil {
				fmt.Println("CONTROLMANAGER: Error decoding control frame:", err.Error())
			} else {
				switch fr.Typ {
				case frame.CTRLUNPAIR:
					fmt.Println("CONTROLMANAGER: Received unpair signal from RPServer!")
					n.paired = false
					return
				case frame.CTRLCONNECT:
					ip := net.ParseIP(n.ctrlConn.RemoteAddr().String())
					port, err := strconv.ParseUint(fr.Data[0], 10, 16)
					if err != nil {
						fmt.Println("CONTROLMANAGER: Error parsing port number:", err.Error())
						continue
					}
					toProxy, err := net.DialTCP("tcp", nil, &net.TCPAddr{IP: ip, Port: int(port)})
					if err != nil {
						fmt.Println("CONTROLMANAGER: Error dialing proxy port:", err.Error())
						continue
					}
					n.handoff(toProxy, fr.Data[1])
				case frame.CTRLHIDETCP:
					//TODO: received after client send exposetcp but server cant expose any more tcp ports. client should hide the port
				}
			}
		}
	}
}

func (n *Networker) handoff(pConn *net.TCPConn, localPort string) {
	// connect to localhost:localPort
	port, err := strconv.ParseUint(localPort, 10, 16)
	if err != nil {
		fmt.Println("CONTROLMANAGER: Error parsing port number:", err.Error())
		return
	}
	lConn, err := net.DialTCP("tcp", nil, &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: int(port)})
	if err != nil {
		fmt.Println("CONTROLMANAGER: Local server is offline:", err.Error())
		return
	}
	defer lConn.Close()
	fmt.Println("CONTROLMANAGER: Connected to local port! Handing off to handlePair...")
	// hand off both to handlePair

	n.wg.Add(1)
	go func(port uint16, pConn *net.TCPConn, lConn *net.TCPConn) {
		defer n.wg.Done()
		defer pConn.Close()
		defer lConn.Close()

		for n.paired && n.expTCP[port] {
			buf := make([]byte, 2048)
			// read from lConn and write to pConn
			lConn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
			m, err := lConn.Read(buf)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					// healthy timeout, continue
					continue
				} else {
					fmt.Println("ERROR: handoff: ", err.Error())
					return
				}
			} else {
				_, err = pConn.Write(buf[:m])
				if err != nil {
					fmt.Println("ERROR: handoff: ", err.Error())
					return
				}
			}
		}
	}(uint16(port), pConn, lConn)

	n.wg.Add(1)
	go func(port uint16, pConn *net.TCPConn, lConn *net.TCPConn) {
		defer n.wg.Done()
		defer pConn.Close()
		defer lConn.Close()
		for n.paired && n.expTCP[port] {
			// read from pConn and write to lConn
			buf := make([]byte, 2048)
			pConn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
			m, err := pConn.Read(buf)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					// healthy timeout, continue
					continue
				} else {
					fmt.Println("ERROR: handoff: ", err.Error())
					return
				}
			} else {
				_, err = lConn.Write(buf[:m])
				if err != nil {
					fmt.Println("ERROR: handoff: ", err.Error())
					return
				}
			}
		}
	}(uint16(port), pConn, lConn)
}

// DONE
func (n *Networker) pair(ip string) error {
	// attempt to dial RPserver on control port
	conn, err := net.DialTCP("tcp", nil, &net.TCPAddr{IP: net.ParseIP(ip), Port: int(frame.CTRLPORT)})
	if err != nil {
		return errors.New("PAIR: error dialing control port")
	}

	// read from control connection
	buf := make([]byte, 1)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf[0], err = bufio.NewReader(conn).ReadByte()
	if err != nil {
		return errors.New("PAIR: error receiving handshake on control connection")
	} else {
		if buf[0] == 'a' {
			fmt.Println("PAIR: Received handshake... Sending authentication...")
			//authentication successful, return connection
			_, err = conn.Write([]byte(frame.TOKEN))
			if err != nil {
				return errors.New("PAIR: failed to send authentication byte")
			}
			n.paired = true
			n.ctrlConn = conn
			n.wg.Add(1)
			go n.manageCtrl()
		} else {
			conn.Close()
			return errors.New("PAIR: wrong handshake received")
		}
	}
	return errors.New("PAIR: unknown error")
}

// DONE
func (n *Networker) unpair() error {
	msg, err := frame.ToByteArray(&frame.CTRLFrame{Typ: frame.CTRLUNPAIR})
	if err != nil {
		return errors.New("UNPAIR: failed to create unpair message")
	} else {
		_, err := n.ctrlConn.Write(msg)
		if err != nil {
			return errors.New("UNPAIR: failed to send unpair message")
		}
		n.paired = false
	}
	return nil
}

// DONE
func (n *Networker) exposeTCP(port uint16) error {
	portStr := strconv.Itoa(int(port))
	msg, err := frame.ToByteArray(&frame.CTRLFrame{Typ: frame.CTRLEXPOSETCP, Data: []string{portStr}})
	if err != nil {
		return errors.New("EXPOSETCP: failed to create expose message")
	}
	n.expTCP[port] = true
	_, err = n.ctrlConn.Write(msg)
	if err != nil {
		return errors.New("EXPOSETCP: failed to send expose message")
	}
	return nil
}

// DONE
func (n *Networker) hideTCP(port uint16) error {
	portStr := strconv.Itoa(int(port))
	msg, err := frame.ToByteArray(&frame.CTRLFrame{Typ: frame.CTRLHIDETCP, Data: []string{portStr}})
	if err != nil {
		return errors.New("EXPOSETCP: failed to create hide message")
	}
	n.expTCP[port] = false
	_, err = n.ctrlConn.Write(msg)
	if err != nil {
		return errors.New("EXPOSETCP: failed to send hide message")
	}
	return nil
}

// TODO: implement
func (n *Networker) exposeUDP(port uint16) error {
	return nil
}

// TODO: implement
func (n *Networker) hideUDP(port uint16) error {
	return nil
}
