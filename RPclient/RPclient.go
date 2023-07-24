package rpclient

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"
)

var run bool = false
var ctrlPort int = 47921
var proxyPort int = 47922
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

func consoleController(csl chan bool) {
	fmt.Println("Welcome to RPclient!")
	var cslString string
	for proceed := false; !proceed; {
		fmt.Println("Please specify the address of your RPserver: ")
		fmt.Scanln(&cslString)
		resolveDomain(cslString)
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
	cslIn := make(chan bool)
	go consoleController(cslIn)
	<-cslIn

	// convert RPserver string into net.IPAddr
	// attempt connection and authentication on RPserver
	// on-success: start relayPackets goroutine
	// on failure, give error on console, exit
}
