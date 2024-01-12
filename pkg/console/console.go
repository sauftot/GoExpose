package console

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

func InputHandler(stop chan<- struct{}, input chan<- []string) {
	var cslString string
	for {
		_, err := fmt.Scanln(&cslString)
		if err != nil {
			close(stop)
			panic("CONSOLECONTROLLER: Couldn't read from console: " + err.Error())
			return
		}
		cslString = strings.ToLower(cslString)
		tokens := strings.Split(cslString, " ")
		switch tokens[0] {
		case "exit":
			fmt.Println("CONSOLECONTROLLER: Received stop command!")
			close(stop)
			return
		default:
			input <- tokens
		}
	}
}

func StopHandler(stop chan<- struct{}) {
	var cslString string
	for {
		_, err := fmt.Scanln(&cslString)
		if err != nil {
			fmt.Println("CONSOLECONTROLLER: Couldn't read from console!")
			close(stop)
			return
		}
		cslString = strings.ToLower(cslString)
		tokens := strings.Split(cslString, " ")
		switch tokens[0] {
		case "exit":
			fmt.Println("CONSOLECONTROLLER: Received stop command!")
			close(stop)
			return
		}
	}
}

func CheckPort(port string) (uint16, error) {
	p, err := strconv.ParseUint(port, 10, 16)
	if err != nil {
		fmt.Println("ERROR: handleInput: invalid port number!")
		return 0, errors.New("invalid port number")
	}
	if p < 8080 || p > 65535 {
		return 0, errors.New("invalid port number")
	} else {
		return uint16(p), nil
	}
}
