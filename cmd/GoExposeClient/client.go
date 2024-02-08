package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

const (
	CTRLPORT int = 47921
)

type Client struct {
	proxy *Proxy

	ctx context.Context
}

func NewClient(context context.Context) *Client {
	return &Client{
		proxy: NewProxy(),
		ctx:   context,
	}
}

func (c *Client) run(input chan []string) {
	defer wg.Done()
	config := c.prepareTlsConfig()
	if config == nil {
		logger.Error("Error preparing TLS config: ", nil)
		return
	}
	c.proxy.setConfig(config)
	logger.Log("Client started")

	for {
		select {
		case <-c.ctx.Done():
			return
		case cmd := <-input:
			c.handleCommand(cmd)
			logger.Log("Command handled")
		}
	}
}

func (c *Client) prepareTlsConfig() *tls.Config {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		logger.Error("Error getting home directory:", err)
		return nil
	}
	keyPath := filepath.Join(homeDir, "certs", "tower.test.key")
	crtPath := filepath.Join(homeDir, "certs", "tower.test.crt")
	cer, err := tls.LoadX509KeyPair(crtPath, keyPath)
	if err != nil {
		logger.Error("Error loading key pair:", err)
		return nil
	}

	config := &tls.Config{
		Certificates:       []tls.Certificate{cer},
		InsecureSkipVerify: true, // The servers certificate is self-signed, the clients is signed by the server. This should be adjusted in the future
	}
	return config
}

func (c *Client) handleCommand(cmd []string) {
	switch cmd[0] {
	case "pair":
		if c.proxy.ctx != nil {
			if c.proxy.ctx.Value("paired") == true {
				fmt.Println("[ERROR] Already paired!")
				return
			} else {
				logger.Error("Error: ctx is not nil but not paired!", nil)
				return
			}
		}
		if len(cmd) != 2 {
			fmt.Println("[ERROR] Invalid Arguments! Use 'pair <server>'")
			return
		}
		c.proxy.connectToServer(cmd[1], c.ctx)
	case "unpair":
		if c.proxy.ctx.Value("paired") == true {
			fmt.Println("[ERROR] Not paired!")
			return
		}
		c.proxy.ctxClose()
	case "expose":
		if c.proxy.ctx.Value("paired") == true {
			fmt.Println("[ERROR] Not paired!")
			return
		}
		port, err := strconv.Atoi(cmd[1])
		if err != nil {
			fmt.Println("[ERROR] Invalid port number!")
			return
		}
		if c.proxy.exposedPortsNr >= 10 {
			fmt.Println("[ERROR] Maximum number of exposed ports reached!")
			return
		}

		if c.proxy.exposedPorts[port] != nil {
			fmt.Println("[ERROR] Port already exposed!")
			return
		}
		c.proxy.expose(port)

		/*
			TODO: Implement contexts properly. Structure: stop ctx -> client ctx -> proxy ctx (paired) -> port ctx
		*/
	case "hide":
		if c.proxy.ctx.Value("paired") == true {
			fmt.Println("[ERROR] Not paired!")
			return
		}
		c.proxy.hide(cmd[1])
	default:
		fmt.Println("[ERROR] Unknown command: ", cmd[0], " use 'pair', 'unpair', 'expose' or 'hide'.")
	}
}
