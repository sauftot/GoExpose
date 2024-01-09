package main

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"time"
)

type Networker struct {
	paired   bool
	ctrlConn *net.TCPConn
}

func newNetworker() *Networker {
	return &Networker{
		paired:   false,
		ctrlConn: nil,
	}
}

func (n *Networker) run(stop <-chan bool) {
	for n.paired {
		select {
		case <-stop:
			//TODO: unpair, stop all relay connections, close ctrlConn connection
		}
	}

}

// DONE
func (n *Networker) pair(ip string) error {
	// attempt to dial RPserver on control port
	conn, err := net.DialTCP("tcp", nil, &net.TCPAddr{IP: net.ParseIP(ip), Port: int(CTRLPORT)})
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
			_, err = conn.Write([]byte("A9os78j8796dfgfmioas87od"))
			if err != nil {
				return errors.New("PAIR: failed to send authentication byte")
			}
			n.paired = true
			n.ctrlConn = conn
		} else {
			conn.Close()
			return errors.New("PAIR: wrong handshake received")
		}
	}
	return errors.New("PAIR: unknown error")
}

// DONE
func (n *Networker) unpair() error {
	msg, err := toByteArray(&CTRLFrame{typ: CTRLUNPAIR})
	if err != nil {
		return errors.New("UNPAIR: failed to create unpair message")
	} else {
		_, err := n.ctrlConn.Write(msg)
		if err != nil {
			return errors.New("UNPAIR: failed to send unpair message")
		}
	}
	return nil
}
