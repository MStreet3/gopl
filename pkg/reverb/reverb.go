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
// specified delay with binary exponential backoff.
func echo(c net.Conn, shout string, delay time.Duration) {
	fmt.Fprintf(c, "\t%s\n", strings.ToUpper(shout))
	time.Sleep(delay * 2)
	fmt.Fprintf(c, "\t%s\n", shout)
	time.Sleep(delay * 4)
	fmt.Fprintf(c, "\t%s\n", strings.ToLower(shout))
}

// heartbeat logs a pulse every 2500 ms until stopped
func heartbeat(stop chan int) {
	for {
		select {
		case <-stop:
			log.Println("stopped heartbeat")
			return

		case <-time.After(2500 * time.Millisecond):
			log.Println("pulse")
		}
	}
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
func serve(stop chan int, addr string) error {
	var (
		wg         sync.WaitGroup
		listener   net.Listener
		done       = make(chan int)
		connStream = make(chan net.Conn)
	)

	// Create a new listener
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return NewReverbServerStartUpError(err)
	}

	// Launch connection handler, stop listening if conn handler stops
	wg.Add(1)
	go func() {
		defer wg.Done()
		handleConnStream(stop, connStream)
		listener.Close()
	}()

	// Launch hearbeat service
	wg.Add(1)
	go func() {
		defer wg.Done()
		heartbeat(stop)
	}()

	// Wait for connection handler to finish to know server is done handling
	go func() {
		defer close(done)
		wg.Wait()
	}()

	// Accept server connections until stopped, exit server when done cleaning up
	log.Println("reverb server started...")
	for {
		select {
		case <-done:
			log.Println("reverb server stopped")
			return nil

		case <-stop:
			log.Println("stopping reverb server")
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

	// Launch server
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := serve(shutdown, addr)
		if err != nil {
			// could not serve, log and shut down
			log.Println(err)
			return
		}
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
			close(shutdown)
		}
	}
}
