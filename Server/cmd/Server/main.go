package main

import (
	server "Server"
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"sync"
	"time"
)

// var loglevel = new(slog.LevelVar)
var consoleLogging = flag.Bool("consolelog", false, "Enable console logging")

/*
	STATUS:
		- 2024-02-10: Proxying is working.
*/

func setupLoggerWriter() io.Writer {
	// check if goexpose directory exists in /var/logger
	if _, err := os.Stat("/var/logger/goexpose"); os.IsNotExist(err) {
		err = os.Mkdir("/var/logger/goexpose", 0755)
		if err != nil {
			panic("Failed to create /var/logger/goexpose directory: " + err.Error())
		}
	}
	// create logger file
	file, err := os.OpenFile("/var/logger/goexpose/server"+time.Now().Format(time.RFC3339)+".logger", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		panic(err)
	}
	var writers []io.Writer
	writers = append(writers, file)
	if *consoleLogging {
		writers = append(writers, os.Stdout)
	}

	return io.MultiWriter(writers...)
}

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

func main() {
	ctx, cnl := context.WithCancel(context.Background())
	defer cnl()
	var testwg sync.WaitGroup

	extGoExpose, extExt := createConnPair(40001)
	defer extGoExpose.Close()
	defer extExt.Close()

	if extGoExpose == nil || extExt == nil {
		panic("Failed to create connections")
	}

	proxGoExpose, proxExt := createConnPair(40000)
	defer proxGoExpose.Close()
	defer proxExt.Close()

	if proxGoExpose == nil || proxExt == nil {
		panic("Failed to create connections")
	}

	dummyconn := &net.TCPConn{}

	p := server.NewProxy(dummyconn, setupTestLogger())

	// TODO: fix waitgroups, somehow they are not working
	testwg.Add(2)
	go p.RelayTcp(proxGoExpose, extGoExpose, ctx, &testwg)
	go p.RelayTcp(extGoExpose, proxGoExpose, ctx, &testwg)

	// give the routine some time to start up
	time.Sleep(100 * time.Millisecond)

	_, err := extExt.Write([]byte("Hello World!"))
	if err != nil {
		panic(err)
	}

	buf := make([]byte, 256)
	n, err := proxExt.Read(buf)
	if err != nil {
		panic(err)
	}

	if string(buf[:n]) != "Hello World!" {
		panic("Data mismatch")
	} else {
		fmt.Println("Data match")
	}

	fmt.Println("Closing connection on external side")

	err = extExt.Close()
	if err != nil {
		panic(err)
	}

	time.Sleep(200 * time.Millisecond)

	fmt.Println("Attempting to write to closed connection on other side")

	_, err = extExt.Write([]byte("Hello World!"))
	if err == nil {
		panic("Expected error, got nil")
	}

	time.Sleep(1 * time.Second)

	fmt.Println("Waiting for routines to close")

	testwg.Wait()

	fmt.Println("TCP Relay test passed")

	/*// Setup logger
	writer := setupLoggerWriter()
	logger := slog.New(slog.NewTextHandler(writer, &slog.HandlerOptions{
		Level: loglevel,
	}))

	// GoExpose Server uses a root context to manage shutting down all goroutines, a sub-context will be derived for each
	// open port that gets relayed by the server
	ctx, cancel := context.WithCancel(context.Background())

	// WaitGroup for synchronisation
	var wg sync.WaitGroup

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	server := Server{
		logger: logger,
	}
	wg.Add(1)
	go server.run(ctx, &wg)

	<-signals
	cancel()
	logger.Info("Received SIGINT/SIGTERM. Closing context and waiting for server to stop...")
	wg.Wait()
	logger.Info("Server stopped")

	*/
}
