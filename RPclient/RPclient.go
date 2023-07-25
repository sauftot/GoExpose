package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var run bool = false
var ctrlPort int = 47921
var proxyPort int = 47922
var localPort int = 8080
var RPserver string = ""
var ip net.IP

func isNumericalOnly(s string) bool {
	for _, char := range s {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func resolveDomain(domain string) (net.IP, error) {
	ipRegex := `^(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})$`
	if match, _ := regexp.MatchString(ipRegex, RPserver); match {
		ip = net.ParseIP(RPserver)
		if ip == nil {
			fmt.Println("Invalid IP address.")
			return nil, errors.New("invalid IP address")
		}
		return ip, nil
	}

	// Check if it's a valid domain name
	domainRegex := `^([a-zA-Z0-9][a-zA-Z0-9-]*\.)+[a-zA-Z]{2,}$`
	if match, _ := regexp.MatchString(domainRegex, RPserver); match {
		ipAddr, err := net.ResolveIPAddr("ip", RPserver)
		if err != nil {
			fmt.Println("Error resolving domain:", err)
			return nil, errors.New("error resolving domain")
		}
		ip = ipAddr.IP
		fmt.Println("Resolved domain to IP:", ip.String())
		return ip, nil
	}
	return nil, errors.New("invalid IP address or domain name")
}

// make user input address of RPserver, and port for local server then wait for stop command
func consoleController(csl chan bool) {
	fmt.Println("Welcome to RPclient!")
	var cslString string
	for proceed := false; !proceed; {
		fmt.Println("Please specify the address of your RPserver: ")
		fmt.Scanln(&cslString)
		_, err := resolveDomain(cslString)
		if err != nil {
			fmt.Println("WARNING: Please enter a valid IP address or domain name!")
		} else {
			for proceed2 := false; !proceed2; {
				fmt.Println("Please specify the port of your application server: ")
				fmt.Scanln(&cslString)
				if isNumericalOnly(cslString) {
					localPort, _ = strconv.Atoi(cslString)
					proceed, proceed2 = true, true
					csl <- true
				} else {
					fmt.Println("WARNING: Please enter a numerical port!")
				}
			}
		}
	}
	fmt.Println("Proceeding, exit the program with the command \"stop\"")
	for run {
		fmt.Scanln(&cslString)
		if strings.ToLower(cslString) == "stop" {
			run = false
			return
		} else {
			fmt.Println("WARNING: Invalid command!")
		}
	}
}

// wait for signals from RPserver then establish proxConn and hand off [proxConn, locConn] to handlePair()
func relayPackets(ctrlConn *net.TCPConn) {
	buf := make([]byte, 7)
	for run {
		ctrlConn.SetDeadline(time.Now().Add(1 * time.Second))
		_, err := ctrlConn.Read(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok {
				// Check if the error is due to a timeout
				if netErr.Timeout() {
					// good timeout, do nothing
				} else {
					// Handle other network-related errors
					fmt.Println("ERROR: Network:", err)
					run = false
					return
				}
			} else {
				// Handle non-network-related errors
				fmt.Println("ERROR: Non-Network:", err)
				run = false
				return
			}
		} else {
			if string(buf) == "extConn" {
				fmt.Println("Received signal from RPserver, attempting connection to proxy...")
				// attempt connection to proxy
				proxConn, err := net.DialTCP("tcp", nil, &net.TCPAddr{IP: ip, Port: proxyPort})
				if err != nil {
					fmt.Println("ERROR: Failed connecting to proxy:", err)
					run = false
					return
				}
				fmt.Println("Connection to proxy established, handing off to handlePair()...")
				go handlePair(*proxConn)
			}
		}
	}
}

// relay packets between proxy and local server
func handlePair(proxConn net.TCPConn) {
	defer proxConn.Close()
	locConn, err := net.DialTCP("tcp", nil, &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: localPort})
	if err != nil {
		fmt.Println("ERROR: Failed connecting to local server:", err)
		return
	}
	defer locConn.Close()
	ch1 := make(chan bool)
	ch2 := make(chan bool)
	go func() {
		buf := make([]byte, 1024)
		for {
			locConn.SetReadDeadline(time.Now().Add(1 * time.Second))
			i, err := locConn.Read(buf)
			if !run {
				return
			}
			if err != nil {
				ch2 <- false
				return
			}
			_, err = proxConn.Write(buf[:i])
			if err != nil {
				ch2 <- true
				return
			}
		}
	}()

	go func() {
		buf := make([]byte, 1024)
		for {
			proxConn.SetReadDeadline(time.Now().Add(1 * time.Second))
			i, err := proxConn.Read(buf)
			if !run {
				return
			}
			if err != nil {
				ch1 <- false
				return
			}
			_, err = locConn.Write(buf[:i])
			if err != nil {
				ch1 <- false
				return
			}
		}
	}()

	<-ch1
	<-ch2
}

func main() {
	run = true
	cslIn := make(chan bool)
	go consoleController(cslIn)
	<-cslIn

	// attempt connection and authentication on RPserver
	ctrlConn, err := net.DialTCP("tcp", nil, &net.TCPAddr{IP: ip, Port: ctrlPort})
	if err != nil {
		fmt.Println("Error dialing RPserver:", err)
		return
	}
	authBuf := make([]byte, 4)
	ctrlConn.Read(authBuf)
	authInt := binary.LittleEndian.Uint32(authBuf)
	authInt = authInt + 5
	binary.LittleEndian.PutUint32(authBuf, authInt)
	_, err = ctrlConn.Write(authBuf)
	if err != nil {
		fmt.Println("Error writing auth to RPserver:", err)
		return
	}

	// on-success: start relayPackets goroutine
	relayPackets(ctrlConn)
}
