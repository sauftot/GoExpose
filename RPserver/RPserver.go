package main

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"
)

const (
	ctrlPort  = 47921
	proxyPort = 47922
)

var (
	appPort = 25565
	wg      sync.WaitGroup
)

func main() {
	welcome()
	fmt.Println("Waiting for control connection...")
	done := make(chan struct{})
	go consoleController(done)

	conn, err := pair(done)
	if err != nil {
		fmt.Println("Error pairing:", err.Error())
		fmt.Println("Shutting down...")
		return
	}
	defer conn.Close()

	proxToCtrl := make(chan string)
	wg.Add(1)
	go controlManager(done, conn, proxToCtrl)
	// the control connection is authenticated and handled, now we can listen for external connections.
	// the program should shut down when the done channel is closed either by you using the stop command,
	// or by the RPclient application closing the control connection

	// CONTINUE HERE!!!
	// controlManager should be done, next is to implement proxyManager which listens for connections from the outside,
	// tells controlManager to connect to the proxy port, and then starts a goroutine to relay packets between each pair of the two connections

	fmt.Println("Listening for external connections on port", appPort)

	wg.Wait()
	fmt.Println("All goroutines finished! Shutting down...")
}

func pair(done <-chan struct{}) (*net.TCPConn, error) {
	// make a listener for the control conn
	l, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.IPv4zero, Port: ctrlPort})
	if err != nil {
		fmt.Println("Error listening:", err)
		return nil, errors.New("error listening")
	}
	defer l.Close()

	// keep accepting connections until we get one from the client or the user stops the server
	buf := make([]byte, 1)
	for {
		select {
		case <-done:
			return nil, errors.New("stop command received before pairing was complete")
		default:
			// accept timeout of 1 second
			err := l.SetDeadline(time.Now().Add(1 * time.Second))
			if err != nil {
				fmt.Println("Error setting deadline:", err)
				return nil, errors.New("error setting deadline")
			}
			conn, err := l.AcceptTCP()
			if err != nil {
				if opErr, ok := err.(*net.OpError); ok && opErr.Timeout() {
					// healthy timeout keep looping
					continue
				}
				fmt.Println("Error accepting:", err)
				return nil, errors.New("error accepting")
			} else {
				// control connection established, authenticate
				fmt.Println("Control connection established. Attempting authentication...")
				conn.Write([]byte("a"))
				conn.SetReadDeadline(time.Now().Add(10 * time.Second))
				_, err := conn.Read(buf)
				if err != nil {
					return nil, errors.New("error reading from control connection")
				} else {
					if buf[0] == 'm' {
						fmt.Println("Authentication successful!")
						return conn, nil
					} else {
						fmt.Println("Authentication failed!")
						conn.Close()
						continue
					}
				}
			}
		}
	}
}

func welcome() {
	fmt.Println("Welcome to RPserver!")
	fmt.Println("Please enter the external Port where you want to expose the server (8081-65534): ")
	var buf string
	proceed := false
	for !proceed {
		fmt.Scanln(&buf)
		intBuf, err := strconv.Atoi(buf)
		if err != nil {
			fmt.Println("Please enter a numerical port number!")
		} else {
			if intBuf > 8080 && intBuf < 65535 {
				appPort = intBuf
				proceed = true
			} else {
				fmt.Println("Please enter a port number between 8080 and 65535!")
			}
		}
	}
}

func consoleController(done chan<- struct{}) {
	for {
		var cslString string
		fmt.Println("Enter \"stop\" to stop the server.")
		fmt.Scanln(&cslString)
		switch cslString {
		case "stop":
			close(done)
			return
		default:
			fmt.Println("Command not recognized! Enter \"stop\" to stop the server.")
		}
	}
}

func controlManager(done chan<- struct{}, ctrl *net.TCPConn, proxToCtrl <-chan string) {
	buf := make([]byte, 1)
	for {
		select {
		case <-proxToCtrl:
			// send signal to controller to connect to proxy port
			_, err := ctrl.Write([]byte("x"))
			if err != nil {
				fmt.Println("Error writing to ctrl:", err)
				close(done)
				return
			}
		default:
			err := ctrl.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
			if err != nil {
				fmt.Println("Error setting ctrl deadline:", err)
				close(done)
				return
			}
			_, err = ctrl.Read(buf)
			if err != nil {
				if opErr, ok := err.(*net.OpError); ok && opErr.Timeout() {
					// healthy timeout keep looping
					continue
				} else {
					fmt.Println("Error reading from ctrl:", err)
					close(done)
					return
				}
			}
		}
	}
}

func proxyManager() {

}
