package Server

type Portqueue struct {
	ports []int
}

// NewPortqueue creates a new Portqueue object with a list of ports from TCPPROXYBASE to TCPPROXYBASE+TCPPROXYAMOUNT
// It functions like a queue, where GetPort returns the first port in the list and removes it from the list.
// ReturnPort adds a port back to the list. A maximum of TCPPROXYAMOUNT ports can be used to proxy ports at a time.
//
// GoExpose Server works by proxying external connections to a GoExpose connection. Once the GoExpose client wants to expose a port,
// the server will assign a proxy port to the external port.
func NewPortqueue() *Portqueue {
	portQ := &Portqueue{
		ports: make([]int, 0, 10),
	}
	for i := range TCPPROXYAMOUNT {
		portQ.ports = append(portQ.ports, TCPPROXYBASE+i)
	}
	return portQ
}

func (pq *Portqueue) GetPort() int {
	if len(pq.ports) == 0 {
		return 0
	}
	port := pq.ports[0]
	pq.ports = pq.ports[1:]
	return port
}

func (pq *Portqueue) ReturnPort(port int) {
	pq.ports = append(pq.ports, port)
}
