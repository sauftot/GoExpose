package main

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"regexp"
	"sync"
	"time"
)

const (
	ctrlPort  int = 47921
	proxyPort int = 47922
)

var (
	localPort int    = 25565
	ip        string = ""
	wg        sync.WaitGroup
)

/*
This is one of the 2 programs making up this reverse proxy project that allows a user to expose a local server to the internet
using a publicly routable small VPS or the likes.
RPclient is the part that runs on your local machine behind a NAT or packet inspecting firewall.
------------------------------------------------------------------------------------------------------------------------
!IMPORTANT! This project only implements basic VERY BASIC authentication, and should not be used in a production environment
or in a public environment without additional security changes, measures and monitoring. This is basically pgrok but worse.
*/
func main() {
	welcome()
	fmt.Println("Establishing control connection (5s Timeout)...")
	conn, err := pair()
	if err != nil {
		fmt.Println("Error pairing:", err.Error())
		fmt.Println("Shutting down...")
		return
	}
	defer conn.Close()

	done := make(chan struct{})
	go consoleController(done)

	controlManager(done, conn)

	fmt.Println("CONTROL MANAGER returned. Shutting down all goroutines...")
	close(done)
	wg.Wait()
	fmt.Println("All goroutines finished! Shutting down.")
}

// welcome literally welcomes the user in the console and then configures the RPclient.
func welcome() {
	fmt.Println("Welcome to RPclient!")
	proceed := false
	for !proceed {
		fmt.Println("Enter the address of your RPserver (Domain or IP): ")
		var serverAddress string
		fmt.Scanln(&serverAddress)
		lip, err := resolveAddress(serverAddress)
		if err != nil {
			fmt.Println(err)
			continue
		} else {
			ip = lip
			for !proceed {
				fmt.Println("Enter the port of your local application server you wish to expose: ")
				fmt.Scanln(&localPort)
				if localPort < 1 || localPort > 65535 {
					fmt.Println("WELCOME: invalid port number!")
					continue
				} else {
					proceed = true
				}
			}
		}
	}

}

// check if input is valid IP or domain, if domain then it resolves the domain to an IP
func resolveAddress(address string) (string, error) {
	ipPattern := `^(?:[0-9]{1,3}\.){3}[0-9]{1,3}$`
	if match, _ := regexp.MatchString(ipPattern, address); match {
		return address, nil
	} else {
		ips, err := net.LookupIP(address)
		if err != nil {
			fmt.Println("RESOLVEADDRESS:", err)
			return "", errors.New("error resolving domain")
		}
		return ips[0].String(), nil
	}
}

// consoleController reads from the console and shuts down the RPclient when the user enters "stop".
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

// pair establishes the control connection and authenticates it. this is the first step in setting up the reverse proxy from the client side.
func pair() (*net.TCPConn, error) {
	// attempt to dial RPserver on control port
	conn, err := net.DialTCP("tcp", nil, &net.TCPAddr{IP: net.ParseIP(ip), Port: ctrlPort})
	if err != nil {
		fmt.Println("PAIR: Error dialing control port:", err.Error())
		return nil, errors.New("error dialing control port")
	}
	// read from control connection
	buf := make([]byte, 1)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	buf[0], err = bufio.NewReader(conn).ReadByte()
	if err != nil {
		fmt.Println("PAIR: Error receiving handshake on control connection")
		return nil, err
	} else {
		if buf[0] == 'a' {
			fmt.Println("Received handshake... Sending authentication...")
			//authentication successful, return connection
			_, err = conn.Write([]byte("m"))
			if err != nil {
				fmt.Println("PAIR: Error handshaking on control connection")
				return nil, errors.New("failed to send authentication byte")
			}
			return conn, nil
		} else {
			fmt.Println("PAIR: Authentication failed")
			conn.Close()
			return nil, errors.New("wrong handshake received")
		}
	}
	// check if received byte is 'm'
}

// controlManager waits for RPserver to signal that there is an outstanding external connection to be relayed.
// this is done through the control connection. It then connects to the proxy port on RPserver and the local server
// and hands those connections off to handlePair for relaying.
func controlManager(done chan struct{}, conn *net.TCPConn) {
	// listen for control messages
	for {
		select {
		case <-done:
			fmt.Println("CONTROLMANAGER: Received stop signal!")
			return
		default:
			conn.SetReadDeadline(time.Now().Add(1 * time.Second))
			buf, err := bufio.NewReader(conn).ReadByte()
			if err != nil {
				if neterr, ok := err.(net.Error); ok && neterr.Timeout() {
					// healthy timeout, continue
					continue
				} else {
					fmt.Println("CONTROLMANAGER: Non-timeout error reading from control connection:", err.Error())
					return
				}
			}
			if buf == 'x' {
				fmt.Println("CONTROLMANAGER: Received proxy signal from RPServer!")
				// connect to RPserver proxPort
				pConn, err := net.DialTCP("tcp", nil, &net.TCPAddr{IP: net.ParseIP(ip), Port: proxyPort})
				if err != nil {
					fmt.Println("CONTROLMANAGER: Error dialing proxy port:", err.Error())
					return
				}
				fmt.Println("CONTROLMANAGER: Connected to proxy port!")
				// connect to localhost:localPort
				lConn, err := net.DialTCP("tcp", nil, &net.TCPAddr{IP: net.IPv4zero, Port: localPort})
				if err != nil {
					fmt.Println("CONTROLMANAGER: Error dialing local port:", err.Error())
					return
				}
				fmt.Println("CONTROLMANAGER: Connected to local port! Handing off to handlePair...")
				// hand off both to handlePair
				handlePair(done, *pConn, *lConn)
			}

		}
	}
}

// relay packets between the proxy connection and the local server
func handlePair(done <-chan struct{}, pConn net.TCPConn, lConn net.TCPConn) {
	wg.Add(1)
	go func(done <-chan struct{}, pConn net.TCPConn, lConn net.TCPConn) {
		defer wg.Done()
		defer pConn.Close()
		defer lConn.Close()
		for {
			select {
			case <-done:
				return
			default:
				// read from lConn and write to pConn
				buf := make([]byte, 2048)
				n, err := lConn.Read(buf)
				if err != nil {
					fmt.Println("HANDLEPAIR: Error reading from lConn:", err)
					return
				}
				_, err = pConn.Write(buf[:n])
				if err != nil {
					fmt.Println("HANDLEPAIR: Error writing to pConn:", err)
					return
				}
			}
		}
	}(done, pConn, lConn)

	wg.Add(1)
	go func(done <-chan struct{}, pConn net.TCPConn, lConn net.TCPConn) {
		defer wg.Done()
		defer pConn.Close()
		defer lConn.Close()
		for {
			select {
			case <-done:
				return
			default:
				// read from pConn and write to lConn
				buf := make([]byte, 2048)
				n, err := pConn.Read(buf)
				if err != nil {
					fmt.Println("HANDLEPAIR: Error reading from pConn:", err)
					return
				}
				_, err = lConn.Write(buf[:n])
				if err != nil {
					fmt.Println("HANDLEPAIR: Error writing to lConn:", err)
					return
				}
			}
		}
	}(done, pConn, lConn)
}
