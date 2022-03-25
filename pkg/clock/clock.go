// Package clock is a simple server that writes the current time to
// a connected client via TCP
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"time"
)

// handleConn accepts a network connection and writes the current time
// each second until the client connection is closed.
func handleConn(c net.Conn, loc *time.Location) {
	defer c.Close()
	for {
		_, err := io.WriteString(c, time.Now().In(loc).Format("Mon Jan _2 2006 15:04:05-07:00\n"))
		if err != nil {
			log.Println("lost connection")
			return
		}

		time.Sleep(1 * time.Second)
	}
}

func main() {

	var (
		port int
		addr string
		tz   string
		loc  *time.Location
	)

	// command line args
	flag.IntVar(&port, "port", 8000, "port to listen for connections on")
	flag.Parse()
	addr = fmt.Sprintf("localhost:%d", port)

	// Get env vars
	if tz = os.Getenv("CLOCK_SERVER_TZ"); tz == "" {
		tz = "America/New_York"
	}

	// Set clock location
	loc, err := time.LoadLocation(tz)
	if err != nil {
		log.Fatal(err)
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("server started...")
	for {
		log.Println("awaiting connections")
		conn, err := listener.Accept()
		if err != nil {
			log.Print(err)
			continue
		}

		log.Println("handling connection")
		go handleConn(conn, loc)
	}

}
