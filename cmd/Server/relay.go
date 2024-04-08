package main

import "context"

type Relay struct {
	proxyPort int
	cnl       context.CancelFunc
}

func (r *Relay) cancel() {
	r.cnl()
}
