package Utils

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func InputHandler(cancel context.CancelFunc, input chan<- []string) {
	reader := bufio.NewReader(os.Stdin)
	for {
		cslString, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("Error reading input:", err)
			return
		}
		lower := strings.ToLower(cslString)
		trimmed := strings.Trim(lower, " \n")
		trimmed2 := strings.TrimSpace(trimmed)
		tokens := strings.Split(trimmed2, " ")
		fmt.Println("CONSOLECONTROLLER: Received command: ", tokens[0])
		switch tokens[0] {
		case "exit":
			fmt.Println("CONSOLECONTROLLER: Received stop command!")
			cancel()
			return
		default:
			input <- tokens
		}
	}
}

func StopHandler(cancel context.CancelFunc) {
	var cslString string
	for {
		_, err := fmt.Scanln(&cslString)
		if err != nil {
			fmt.Println("CONSOLECONTROLLER: Couldn't read from console!")

			return
		}
		cslString = strings.ToLower(cslString)
		tokens := strings.Split(cslString, " ")
		switch tokens[0] {
		case "exit":
			fmt.Println("CONSOLECONTROLLER: Received stop command!")
			cancel()
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
