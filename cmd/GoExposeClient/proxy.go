package main

import (
	"crypto/tls"
	"fmt"
	"net"
	"strconv"
)

type Proxy struct {
	Paired bool
	ip     net.IP
	config *tls.Config
}

func NewProxy() *Proxy {
	return &Proxy{
		Paired: false,
		ip:     nil,
		config: nil,
	}
}

func (p *Proxy) setConfig(config *tls.Config) {
	p.config = config
}

func (p *Proxy) connectToServer(domainOrIp string) {
	ip := net.ParseIP(domainOrIp)
	if ip == nil {
		ip2, err := net.ResolveIPAddr("ip4", domainOrIp)
		if err != nil {
			logger.Error("Error resolving domain name: ", err)
			return
		}
		p.ip = ip2.IP
	} else {
		p.ip = ip
	}
	fmt.Println("[INFO] Connecting to server: ", p.ip.String())
	conn, err := tls.Dial("tcp", p.ip.String()+":"+strconv.Itoa(CTRLPORT), p.config)
	if err != nil {
		logger.Error("Error connecting to server: ", err)
		return
	}

}
