package rpserver

import (
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"time"
)

var ctrlPort = 47921
var proxyPort = 47922
var appPort = 8080
var run = false

func isNumericalOnly(s string) bool {
	for _, char := range s {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

// continously relay packets between the two connections until one peer shuts down
func handlePair(proxConn net.TCPConn, extConn net.TCPConn) {
	defer proxConn.Close()
	defer extConn.Close()
	ch1 := make(chan bool)
	ch2 := make(chan bool)
	go func() {
		buf := make([]byte, 1024)
		for {
			proxConn.SetReadDeadline(time.Now().Add(1 * time.Second))
			i, err := extConn.Read(buf)
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
			_, err = extConn.Write(buf[:i])
			if err != nil {
				ch1 <- false
				return
			}
		}
	}()

	<-ch1
	<-ch2
}

func relayPackets(ctrlConn net.Conn, status chan bool, csl2 chan bool) {
	listener, err := net.Listen("tcp", ":"+strconv.Itoa(appPort))
	if err != nil {
		log.Fatal(err)
		status <- true
		return
	}
	defer listener.Close()

	proxL, err2 := net.Listen("tcp", ":"+strconv.Itoa(proxyPort))
	if err2 != nil {
		log.Fatal(err)
		status <- true
		return
	}
	defer listener.Close()

	tcpListener, ok := listener.(*net.TCPListener)
	if !ok {
		fmt.Println("ERROR: Listener is not a TCPListener")
		status <- true
		return
	}

	tcpProxL, ok := proxL.(*net.TCPListener)
	if !ok {
		fmt.Println("ERROR: Listener is not a TCPListener")
		status <- true
		return
	}

	for {
		select {
		case <-csl2:
			status <- true
			return
		default:
		}

		tcpListener.SetDeadline(time.Now().Add(2 * time.Second))
		extConn, err := tcpListener.AcceptTCP()
		if err != nil {
			if netErr, ok := err.(net.Error); ok {
				// Check if the error is due to a timeout
				if netErr.Timeout() {
					// good timeout, do nothing
				} else {
					// Handle other network-related errors
					fmt.Println("ERROR: Network:", err)
				}
			} else {
				// Handle non-network-related errors
				fmt.Println("ERROR: Non-Network:", err)
			}
		} else {
			// tell RPagent that an external connection has been established
			_, err := ctrlConn.Write([]byte("extConn"))
			if err != nil {
				fmt.Println("ERROR: Failed writing to RPagent.")
				status <- true
				return
			}

			tcpProxL.SetDeadline(time.Now().Add(10 * time.Second))
			proxConn, err := tcpProxL.AcceptTCP()
			if err != nil {
				if netErr, ok := err.(net.Error); ok {
					// Check if the error is due to a timeout
					if netErr.Timeout() {
						// good timeout, do nothing
					} else {
						// Handle other network-related errors
						fmt.Println("Network error:", err)
						status <- true
						return
					}
				} else {
					// Handle non-network-related errors
					fmt.Println("Error accepting connection:", err)
					status <- true
					return
				}
			} else {
				go handlePair(*proxConn, *extConn)
			}
		}
	}
}

func consoleController(csl chan bool, csl2 chan bool) {
	fmt.Println("Welcome to RPServer!")
	var cslString string
	for proceed := false; !proceed; {
		fmt.Println("Please enter the port you want to forward to the internet: ")
		fmt.Scanln(&cslString)
		if isNumericalOnly(cslString) {
			proceed = true
			csl <- true
			appPort, _ = strconv.Atoi(cslString)
		} else {
			fmt.Println("WARNING: Please enter a numerical port!")
		}
	}
	fmt.Println("Proceeding, exit the program with the command \"stop\"")
	for {
		fmt.Scanln(&cslString)
		if strings.ToLower(cslString) == "stop" {
			csl <- false
			csl2 <- false
			run = false
			return
		} else {
			fmt.Println("WARNING: Invalid command!")
		}
	}
}

// open a tcp server on a port defined by a global variable
// and wait for a connection to arrive then start a goroutine that lasts until the connection is closed
func main() {
	// create console logger and interface
	csl := make(chan bool)
	csl2 := make(chan bool)
	run = true
	go consoleController(csl, csl2)

	<-csl

	// create a listener
	listener, err := net.Listen("tcp", ":"+strconv.Itoa(ctrlPort))
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()

	tcpListener, ok := listener.(*net.TCPListener)
	if !ok {
		fmt.Println("Listener is not a TCPListener")
		return
	}

	status := make(chan bool)

	var conn net.TCPConn
	defer conn.Close()
	// accept connections
	for {
		tcpListener.SetDeadline(time.Now().Add(2 * time.Second))
		conn, err := tcpListener.AcceptTCP()
		select {
		case <-csl:
			return
		default:
		}
		if err != nil {
			if netErr, ok := err.(net.Error); ok {
				// Check if the error is due to a timeout
				if netErr.Timeout() {
					// good timeout, do nothing
				} else {
					// Handle other network-related errors
					fmt.Println("Network error:", err)
				}
			} else {
				// Handle non-network-related errors
				fmt.Println("Error accepting connection:", err)
			}
		} else {
			// generate a 32 bit unsigned random int
			n := rand.Uint32()
			if n > (math.MaxUint32 - 6) {
				n = math.MaxUint32 - 6
			}
			ba := make([]byte, 4)
			binary.LittleEndian.PutUint32(ba, n)
			_, err := conn.Write(ba)
			if err != nil {
				fmt.Println("ERROR: Failed writing to RPagent.")
			} else {
				buf := make([]byte, 4)
				conn.SetReadDeadline(time.Now().Add(10 * time.Second))
				i, err := conn.Read(buf)
				if err != nil {
					fmt.Println("ERROR: Reading authenticator from RPagent failed.")
				} else if i != 32 {
					fmt.Println("ERROR: Wrong number of bytes received from RPagent.")
				} else {
					if binary.LittleEndian.Uint32(buf) == n-5 {
						go relayPackets(conn, status, csl2)
						<-status
					}
				}
			}
		}
	}
}
