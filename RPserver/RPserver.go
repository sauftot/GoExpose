package main

import (
	"bufio"
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

/*
This is one of the 2 programs making up this reverse proxy project that allows a user to expose a local server to the internet
using a publicly routable small VPS or the likes.
RPserver is the part that runs on the publicly routable server, and RPclient is the part that runs on the local server.
------------------------------------------------------------------------------------------------------------------------
!IMPORTANT! This project only implements basic VERY BASIC authentication, and should not be used in a production environment
or in a public environment without additional security changes, measures and monitoring. This is basically pgrok but worse.
*/
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

	fmt.Println("Starting controlManager...")
	proxToCtrl := make(chan string)
	go controlManager(done, conn, proxToCtrl)

	fmt.Println("Starting proxyManager...")
	if !proxyManager(done, proxToCtrl) {
		close(done)
	}

	fmt.Println("PROXYMANAGER returned. Shutting down all goroutines...")
	wg.Wait()

	fmt.Println("All goroutines finished! Shutting down.")
}

// pair establishes the control connection. this is the first step in setting up the reverse proxy.
// it will listen for a connection to the controlPort from the RPclient, authenticate it and then return it to main.
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
			return nil, errors.New("PAIR: stop command received before pairing was complete")
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
				buf[0], err = bufio.NewReader(conn).ReadByte()
				if err != nil {
					return nil, errors.New("error reading from control connection")
				} else {
					if buf[0] == 'm' {
						fmt.Println("Authentication successful!")
						//authentication successful, return connection
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

// welcome literally welcomes the user in the console and then configures the RPserver.
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

// consoleController reads from the console and sends a signal to the done channel when the user enters "stop".
// this is used to shut down the RPserver.
func consoleController(done chan<- struct{}) {
	for {
		var cslString string
		fmt.Println("Enter \"stop\" to stop the server.")
		fmt.Scanln(&cslString)
		switch cslString {
		case "stop":
			fmt.Println("CONSOLECONTROLLER: Received stop command!")
			close(done)
			return
		default:
			fmt.Println("Command not recognized! Enter \"stop\" to stop the server.")
		}
	}
}

// the purpose of the controlManager is to relay the necessity for a proxyConnection from the proxyManager to the RPclient.
// it also reads from the control connection to make sure it is still alive without actually using anything read from it.
func controlManager(done chan<- struct{}, ctrl *net.TCPConn, proxToCtrl <-chan string) {
	buf := make([]byte, 1)
	for {
		select {
		case i := <-proxToCtrl:
			// send signal to controller to connect to proxy port
			fmt.Println("CONTROLMANAGER: Received signal from proxyManager, relaying via ctrlConn...")
			_, err := ctrl.Write([]byte(i))
			if err != nil {
				fmt.Println("Error writing to ctrl:", err)
				close(done)
				return
			}
			fmt.Println("CONTROLMANAGER: Signal relayed successfully!")
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

// proxyManager is called when the control connection is established and authenticated.
// it will then listen for external connections from clients, tell the controlManager to notify
// RPclient to open a TCP connection to the proxy port, and then start a goroutine to relay packets
func proxyManager(done <-chan struct{}, proxToCtrl chan<- string) bool {
	defer wg.Done()
	eL, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.IPv4zero, Port: appPort})
	if err != nil {
		fmt.Println("PROXYMANAGER: Error listening on appPort:", err)
		return false
	}
	defer eL.Close()
	fmt.Println("Listening for external connections on port", appPort)

	pL, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.IPv4zero, Port: proxyPort})
	if err != nil {
		fmt.Println("PROXYMANAGER: Error listening on proxyPort:", err)
		return false
	}
	defer pL.Close()
	fmt.Println("Listening for proxy connections on port", proxyPort)
	for {
		select {
		case <-done:
			return true
		default:
			// accept external connections
			err := eL.SetDeadline(time.Now().Add(500 * time.Millisecond))
			if err != nil {
				fmt.Println("PROXYMANAGER: Error setting eL deadline:", err)
				return false
			}
			eConn, err := eL.AcceptTCP()
			if err != nil {
				if opErr, ok := err.(*net.OpError); ok && opErr.Timeout() {
					// healthy timeout keep looping
					continue
				} else {
					fmt.Println("PROXYMANAGER: Error accepting eConn:", err)
					return false
				}
			} else {
				defer eConn.Close()
				fmt.Println("PROXYMANAGER: Received external connection, sending signal to controlManager...")

				// send signal to controlManager to connect to proxy port
				proxToCtrl <- "x"
				err := pL.SetDeadline(time.Now().Add(10 * time.Second))
				if err != nil {
					fmt.Println("PROXYMANAGER: Error setting pL deadline:", err)
					return false
				}
				pConn, err := pL.AcceptTCP()
				if err != nil {
					if opErr, ok := err.(*net.OpError); ok && opErr.Timeout() {
						// unhealthy timeout, RPclient did not respond with a proxy connection after being asked via control connection.
						fmt.Println("PROXYMANAGER: RPclient did not respond with a proxy connection after being asked via control connection.")
						eConn.Close()
						continue
					} else {
						fmt.Println("PROXYMANAGER: Error accepting pConn:", err)
						return false
					}
				} else {
					fmt.Println("PROXYMANAGER: Received proxy connection, handing off to handlePair()...")
					handlePair(done, eConn, pConn)
				}
			}
		}
	}
}

// relay packets between the external connection and the proxy connection
func handlePair(done <-chan struct{}, eConn *net.TCPConn, pConn *net.TCPConn) {
	wg.Add(1)
	go func(done <-chan struct{}, eConn *net.TCPConn, pConn *net.TCPConn) {
		defer wg.Done()
		defer pConn.Close()
		defer eConn.Close()
		for {
			select {
			case <-done:
				return
			default:
				// read from eConn and write to pConn
				buf := make([]byte, 2048)
				n, err := eConn.Read(buf)
				if err != nil {
					fmt.Println("HANDLEPAIR: Error reading from eConn:", err)
					return
				}
				_, err = pConn.Write(buf[:n])
				if err != nil {
					fmt.Println("HANDLEPAIR: Error writing to pConn:", err)
					return
				}
			}
		}
	}(done, eConn, pConn)

	wg.Add(1)
	go func(done <-chan struct{}, eConn *net.TCPConn, pConn *net.TCPConn) {
		defer wg.Done()
		defer pConn.Close()
		defer eConn.Close()
		for {
			select {
			case <-done:
				return
			default:
				// read from pConn and write to eConn
				buf := make([]byte, 2048)
				n, err := pConn.Read(buf)
				if err != nil {
					fmt.Println("HANDLEPAIR: Error reading from pConn:", err)
					return
				}
				_, err = eConn.Write(buf[:n])
				if err != nil {
					fmt.Println("HANDLEPAIR: Error writing to eConn:", err)
					return
				}
			}
		}
	}(done, eConn, pConn)
}
