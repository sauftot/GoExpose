package main

type Portqueue struct {
	ports []int
}

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
