package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net"
	"strings"
	"time"
)

func echo(c net.Conn, shout string, delay time.Duration) {
	fmt.Fprintf(c, "\t%s\n", strings.ToUpper(shout))
	time.Sleep(delay)
	fmt.Fprintf(c, "\t%s\n", shout)
	time.Sleep(delay)
	fmt.Fprintf(c, "\t%s\n", strings.ToLower(shout))
}

func handleConn(c net.Conn) {
	input := bufio.NewScanner(c)
	for input.Scan() {
		go echo(c, input.Text(), 1*time.Second)
	}
	c.Close()
}

func main() {

	var (
		port int
		addr string
	)

	// command line args
	flag.IntVar(&port, "port", 8000, "port to listen for connections on")
	flag.Parse()
	addr = fmt.Sprintf("localhost:%d", port)

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
		go handleConn(conn)
	}

}
