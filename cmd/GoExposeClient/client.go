package main

import (
	"crypto/tls"
	"os"
	"path/filepath"
)

type Client struct {
}

func NewClient() *Client {
	return &Client{}
}

func (c *Client) run() {

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
