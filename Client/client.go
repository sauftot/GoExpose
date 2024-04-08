package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"path/filepath"
)

const (
	CTRLPORT string = "47921"
)

type Client struct {
	proxy       *Proxy
	proxyCancel context.CancelFunc

	ctx       context.Context
	tlsConfig *tls.Config
}

func NewClient(context context.Context) *Client {
	return &Client{
		proxy: nil,
		ctx:   context,
	}
}

func (c *Client) run(input chan []string) {
	defer wg.Done()
	c.tlsConfig = c.prepareTlsConfig()
	if c.tlsConfig == nil {
		logger.Error("Error preparing TLS config: ", nil)
		return
	}
	logger.Log("Client started")

	for {
		select {
		case <-c.ctx.Done():
			return
		case cmd := <-input:
			logger.Log("Command received: " + fmt.Sprintf("Command received: %v", cmd))
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
	logger.Log("TLS config prepared")
	return config
}

func (c *Client) handleCommand(cmd []string) {
	switch cmd[0] {
	case "pair":
		if len(cmd) != 2 {
			fmt.Println("[ERROR] Usage: pair <server>")
			return
		}
		if c.proxy != nil {
			fmt.Println("[ERROR] Proxy already paired with server")
			return
		}
		ip := net.ParseIP(cmd[1])
		if ip == nil {
			i, err := net.ResolveIPAddr("ip4", cmd[1])
			ip = i.IP
			if err != nil {
				fmt.Println("[ERROR] Invalid server address")
				logger.Error("Error resolving domain name: ", err)
				return
			}
		}
		ct := context.WithValue(c.ctx, "ip", ip)
		/*
			The pairingContext is live for the duration of the client being paired to a server.
		*/
		pairingCtx, cancel := context.WithCancel(ct)
		c.proxyCancel = cancel
		c.proxy = NewProxy(pairingCtx, cancel, c.tlsConfig)
		if !c.proxy.connectToServer() {
			logger.Error("Error connecting to server", nil)
			c.proxyCancel()
			c.proxy = nil
		}
	case "unpair":
		if c.proxy == nil {
			fmt.Println("[ERROR] Proxy not paired with server")
			return
		}
		c.proxyCancel()
		c.proxy = nil
	case "expose":
		if c.proxy == nil {
			fmt.Println("[ERROR] Proxy not paired with server")
			return
		}
		if len(cmd) != 2 {
			fmt.Println("[ERROR] Usage: expose <port>")
			return
		}
		c.proxy.expose(cmd[1])
	case "hide":
		if c.proxy == nil {
			fmt.Println("[ERROR] Proxy not paired with server")
			return
		}
		if len(cmd) != 2 {
			fmt.Println("[ERROR] Usage: hide <port>")
			return
		}
		c.proxy.hide(cmd[1])
	default:
		fmt.Println("[ERROR] Unknown command: ", cmd[0], " use 'pair', 'unpair', 'expose' or 'hide'.")
	}
}
