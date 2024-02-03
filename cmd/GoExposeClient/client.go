package main

import (
	"crypto/tls"
	"fmt"
	"os"
	"path/filepath"
)

const (
	CTRLPORT int = 47921
)

type Client struct {
	proxy *Proxy
}

func NewClient() *Client {
	return &Client{
		proxy: NewProxy(),
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
		case <-stop:
			return
		case cmd := <-input:
			c.handleCommand(cmd)
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
		if c.proxy.Paired {
			fmt.Println("[ERROR] Already paired!")
			return
		}
		if len(cmd) != 2 {
			fmt.Println("[ERROR] Invalid Arguments! Use 'pair <server>'")
			return
		}
		c.proxy.connectToServer(cmd[1])
	case "unpair":
		if !c.proxy.Paired {
			fmt.Println("[ERROR] Not paired!")
			return
		}
		c.proxy.Paired = false
	case "expose":
		if !c.proxy.Paired {
			fmt.Println("[ERROR] Not paired!")
			return
		}
		c.proxy.expose(cmd[1])
	case "hide":
		if !c.proxy.Paired {
			fmt.Println("[ERROR] Not paired!")
			return
		}
		c.proxy.hide(cmd[1])
	default:
		fmt.Println("[ERROR] Unknown command: ", cmd[0], " use 'pair', 'unpair', 'expose' or 'hide'.")
	}
}
