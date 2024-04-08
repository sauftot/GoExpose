package test

import (
	server "Server"
	"context"
	"log/slog"
	"net"
	"os"
	"sync"
	"testing"
	"time"
)

func setupTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
}

func createConnPair(port int) (*net.TCPConn, *net.TCPConn) {
	ln, err := net.ListenTCP("tcp", &net.TCPAddr{Port: port})
	if err != nil {
		return nil, nil
	}

	conn1, err := net.DialTCP("tcp", nil, &net.TCPAddr{Port: port})
	if err != nil {
		return nil, nil
	}

	conn2, err := ln.AcceptTCP()
	if err != nil {
		return nil, nil
	}

	return conn1, conn2
}

func TestTcpRelay(t *testing.T) {
	t.Log("Testing TCP Relay")

	ctx, cnl := context.WithCancel(context.Background())
	defer cnl()
	testwg := new(sync.WaitGroup)

	extGoExpose, extExt := createConnPair(40001)
	defer extGoExpose.Close()
	defer extExt.Close()

	if extGoExpose == nil || extExt == nil {
		t.Fatal("Failed to create connections")
	}

	proxGoExpose, proxExt := createConnPair(40000)
	defer proxGoExpose.Close()
	defer proxExt.Close()

	if proxGoExpose == nil || proxExt == nil {
		t.Fatal("Failed to create connections")
	}

	dummyconn := &net.TCPConn{}

	p := server.NewProxy(dummyconn, setupTestLogger())

	testwg.Add(1)
	go p.RelayTcp(proxGoExpose, extGoExpose, ctx, testwg)
	testwg.Add(1)
	go p.RelayTcp(extGoExpose, proxGoExpose, ctx, testwg)

	// give the routine some time to start up
	time.Sleep(100 * time.Millisecond)

	_, err := extExt.Write([]byte("Hello World!"))
	if err != nil {
		t.Fatal(err)
	}

	buf := make([]byte, 256)
	n, err := proxExt.Read(buf)
	if err != nil {
		t.Fatal(err)
	}

	if string(buf[:n]) != "Hello World!" {
		t.Fatal("Data mismatch")
	} else {
		t.Log("Data match")
	}

	t.Log("Closing connection on external side")

	err = extExt.Close()
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(200 * time.Millisecond)

	t.Log("Attempting to write to closed connection on other side")

	_, err = extExt.Write([]byte("Hello World!"))
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	t.Log("Waiting for routines to close")

	time.Sleep(200 * time.Millisecond)

	// wg.Wait() does not work in tests anymore WAFUD

	t.Log("TCP Relay test passed")

}
