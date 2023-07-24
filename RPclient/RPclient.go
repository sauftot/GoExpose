package rpclient

import (
	"fmt"
	"net"
	"strings"
)

var run bool = false
var ctrlPort int = 47921
var proxyPort int = 47922
var RPserver string = ""

func isNumericalOnly(s string) bool {
	for _, char := range s {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func consoleController(csl chan string) {
	fmt.Println("Welcome to RPclient!")
	var cslString string
	for proceed := false; !proceed; {
		fmt.Println("Please specify the address of your RPserver: ")
		fmt.Scanln(&cslString)
		// verify input, resolve domain, send address into csl
	}
	fmt.Println("Proceeding, exit the program with the command \"stop\"")
	for {
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
func relayPackets() {
	//
}

// relay packets between proxy and local server
func handlePair(locConn net.TCPConn, proxConn net.TCPConn) {

}

func main() {
	run = true
	cslIn := make(chan string)
	go consoleController(cslIn)
	RPserver = <-cslIn
	// convert RPserver string into net.IPAddr
	// attempt connection and authentication on RPserver
	// on-success: start relayPackets goroutine
	// on failure, give error on console, exit
}
