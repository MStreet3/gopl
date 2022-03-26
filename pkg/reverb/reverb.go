// Package reverb implements a simple server that echoes back input read from a connection.  The
// server gracefully shuts down in the event of a system interruption.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"
)

// echo writes three values of shout to the given connection.  Each value is written after a
// specified delay.
func echo(c net.Conn, shout string, delay time.Duration) {
	fmt.Fprintf(c, "\t%s\n", strings.ToUpper(shout))
	time.Sleep(delay)
	fmt.Fprintf(c, "\t%s\n", shout)
	time.Sleep(delay)
	fmt.Fprintf(c, "\t%s\n", strings.ToLower(shout))
}

// handleConn scans the connection and converts the found input into text for echoing back on the
// connection.
func handleConn(c net.Conn) {
	input := bufio.NewScanner(c)
	for input.Scan() {
		go echo(c, input.Text(), 1*time.Second)
	}
	c.Close()
}

// handleConnStream handles each connection it pulls from a connection stream and stops when
// the stop channel is closed.
func handleConnStream(stop chan int, stream chan net.Conn) {
	for {
		select {
		case <-stop:
			log.Println("closing reverb client connection handler")
			return

		case conn := <-stream:
			log.Println("handling connection")
			go handleConn(conn)
		}
	}
}

// serve launches a listener that waits for new connections and places those connections on a
// connection stream.  serve shuts itself down once the stop channel is closed.
func serve(stop chan int, listener net.Listener) {
	var (
		wg         sync.WaitGroup
		done       = make(chan int)
		connStream = make(chan net.Conn)
	)

	wg.Add(1)
	go func() {
		defer wg.Done()
		handleConnStream(stop, connStream)
	}()

	go func() {
		defer close(done)
		wg.Wait()
	}()

	log.Println("reverb server started...")
	for {
		select {
		case <-done:
			log.Println("reverb server stopped")
			return

		case <-stop:
			log.Println("stopping reverb server")
			continue

		default:
			log.Println("awaiting connections")
			conn, err := listener.Accept()
			if err != nil {
				log.Println(err)
				continue
			}

			connStream <- conn
		}
	}
}

// main launches a reverb server and waits for a system interrupt to gracefully shutdown the reverb
// server.
func main() {

	var (
		port      int
		addr      string
		wg        sync.WaitGroup
		interrupt = make(chan os.Signal, 1)
		shutdown  = make(chan int)
		done      = make(chan int)
	)

	// Notify main of any interruptions
	signal.Notify(interrupt, os.Interrupt)

	// Fetch command line args
	flag.IntVar(&port, "port", 8000, "port to listen for connections on")
	flag.Parse()
	addr = fmt.Sprintf("localhost:%d", port)

	// Create a new listener
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}

	// Launch server
	wg.Add(1)
	go func() {
		defer wg.Done()
		serve(shutdown, listener)
	}()

	// Launch shutdown watcher
	go func() {
		wg.Wait()
		close(done)
	}()

	// Handle graceful shutdown
	for {
		select {
		case <-done:
			log.Println("shutdown complete, goodbye")
			return
		case <-interrupt:
			log.Println("starting graceful shutdown")
			listener.Close()
			close(shutdown)
		}
	}
}
