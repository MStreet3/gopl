package echo

import (
	"io"
	"log"
	"net"
	"os"
)

// mustCopy exits the main routine if there is an error in copying the read
// stream to stdout
func mustCopy(dst io.Writer, src io.Reader) {
	if _, err := io.Copy(dst, src); err != nil {
		log.Fatal(err)
	}
}

// main connects to a server via TCP and copies the read stream of the
// server to stdout
func main() {
	conn, err := net.Dial("tcp", "localhost:8000")
	if err != nil {
		log.Fatal(err)
	}

	defer conn.Close()
	mustCopy(os.Stdout, conn)
}
