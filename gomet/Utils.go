package gomet

import (
	"bufio"
	"github.com/abiosoft/ishell"
	"github.com/xtaci/smux"
	"io"
	"log"
	"math/rand"
	"net"
	"regexp"
	"strings"
	"sync"
)

func acceptNewStream(session *smux.Session) *smux.Stream {
	stream, err := session.AcceptStream()
	if err != nil {
		log.Printf("ERROR %s", err)
		return nil
	}
	log.Printf("New stream opened")
	return stream
}

func readParameter(c *ishell.Context, name string) string {
	c.Print(name)
	return c.ReadLine()
}

func handleConnection(conn net.Conn, stream *smux.Stream, registry *Registry) {

	registry.Register(stream)

	defer conn.Close()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		io.Copy(stream, conn)
		log.Printf("Close input")
		stream.Close()
		wg.Done()
	}()

	go func() {
		io.Copy(conn, stream)
		log.Printf("Close output")
		conn.Close()
		wg.Done()
	}()

	wg.Wait()

	log.Printf("Connection closed")

	registry.Unregister(stream)
}

const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func randomString(length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = alphabet[rand.Intn(len(alphabet))]
	}
	return string(b)
}

func stringMatch(regex string, value []byte) bool {
	match, _ := regexp.Match(regex, value)
	return match
}

func readHttpHeaders(reader *bufio.Reader) map[string] string {
	headers := make(map[string] string)
	for {
		line, _, err := reader.ReadLine()
		if err != nil {
			break
		}

		if len(line) == 0 {
			break
		}

		array := strings.SplitN(string(line),":", 2)
		if len(array) == 2 {
			headers[strings.ToLower(strings.TrimSpace(array[0]))] = strings.TrimSpace(array[1])
		}
	}
	return headers
}


