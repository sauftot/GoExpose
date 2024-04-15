package test

import (
	server "Server"
	"context"
	"log/slog"
	"net"
	"os"
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
		panic(err)
		return nil, nil
	}

	conn1, err := net.DialTCP("tcp", nil, &net.TCPAddr{Port: port})
	if err != nil {
		panic(err)
		return nil, nil
	}

	conn2, err := ln.AcceptTCP()
	if err != nil {
		panic(err)
		return nil, nil
	}

	return conn1, conn2
}

// TestTcpRelay tests the TCP relay functionality.
// It creates two pairs of connections, each one has an external and a proxy side.
// The goal of this test is to check if the data is being relayed correctly between the two external connections.
// This data passes through the RelayTcp function which uses io.Copy to relay the data.
func TestTcpRelayDouble(t *testing.T) {
	t.Log("Testing TCP Relay 2")

	ctx, cnl := context.WithCancel(context.Background())
	defer cnl()

	extGoExpose, extExt := createConnPair(40001)
	if extGoExpose == nil || extExt == nil {
		t.Fatal("Failed to create connections on port 40001")
	}
	defer extGoExpose.Close()
	defer extExt.Close()

	proxGoExpose, proxExt := createConnPair(40000)
	if proxGoExpose == nil || proxExt == nil {
		t.Fatal("Failed to create connections on port 40000")
	}
	defer proxGoExpose.Close()
	defer proxExt.Close()

	dummyconn := &net.TCPConn{}

	p := server.NewProxy(dummyconn, setupTestLogger())

	go p.RelayTcp(extGoExpose, proxGoExpose, ctx)
	go p.RelayTcp(proxGoExpose, extGoExpose, ctx)

	// give the routine some time to start up
	time.Sleep(300 * time.Millisecond)

	t.Log("Writing data to external side")

	_, err := proxExt.Write([]byte("Hello World!"))
	if err != nil {
		t.Fatal(err)
	}

	t.Log("Reading data from proxy external side")

	buf := make([]byte, 256)
	n, err := extExt.Read(buf)
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

	t.Log("Asserting that RelayTcp closed both proxGoExpose and proxExt")

	_, err = proxGoExpose.Read(buf)
	if err == nil {
		t.Fatal("Expected error on proxGoExpose read, got nil")
	}

	_, err = proxExt.Read(buf)
	if err == nil {
		t.Fatal("Expected error on proxExt read, got nil")
	}

	t.Log("Attempting to write to closed connection on other side")

	_, err = proxExt.Write([]byte("Hello World!"))
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	t.Log("TCP Relay test passed")
}
