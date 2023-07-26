package main

import (
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

	wg.Wait()
}

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
	_, err = conn.Read(buf)
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
