package antivirus

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

func scannerServer(t *testing.T, handler func(net.Conn)) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			defer conn.Close()
			handler(conn)
		}
	}()
	return listener.Addr().String()
}

func TestStreamingScanProtocol(t *testing.T) {
	for _, test := range []struct {
		name, response   string
		infected, failed bool
	}{
		{"clean", "stream: OK\x00", false, false},
		{"infected", "stream: Test.Signature FOUND\x00", true, true},
		{"scanner error", "INSTREAM size limit exceeded. ERROR\x00", false, true},
		{"truncated reply", "stream: OK", false, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			received := make(chan string, 1)
			address := scannerServer(t, func(conn net.Conn) {
				reader := bufio.NewReader(conn)
				command, _ := reader.ReadString(0)
				if command != "zINSTREAM\x00" {
					received <- "wrong command"
					return
				}
				var content strings.Builder
				for {
					var length uint32
					if binary.Read(reader, binary.BigEndian, &length) != nil {
						received <- "invalid chunk"
						return
					}
					if length == 0 {
						break
					}
					data := make([]byte, length)
					if _, err := io.ReadFull(reader, data); err != nil {
						received <- "short chunk"
						return
					}
					content.Write(data)
				}
				received <- content.String()
				io.WriteString(conn, test.response)
			})
			content := strings.Repeat("product reference", 5000)
			err := (Clamd{Address: address, Timeout: time.Second}).Scan(context.Background(), strings.NewReader(content))
			if (err != nil) != test.failed || errors.Is(err, ErrInfected) != test.infected {
				t.Fatalf("scan result=%v", err)
			}
			if got := <-received; got != content {
				t.Fatal("scanner did not receive the exact file")
			}
		})
	}
}

func TestHealthChecksStreamingCapability(t *testing.T) {
	address := scannerServer(t, func(conn net.Conn) {
		command, _ := bufio.NewReader(conn).ReadString(0)
		if command == "zVERSIONCOMMANDS\x00" {
			io.WriteString(conn, "ClamAV test| COMMANDS: PING INSTREAM VERSIONCOMMANDS\x00")
		}
	})
	if err := (Clamd{Address: address, Timeout: time.Second}).Health(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestCancelledScanFailsClosed(t *testing.T) {
	address := scannerServer(t, func(conn net.Conn) { io.Copy(io.Discard, conn) })
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := (Clamd{Address: address, Timeout: time.Second}).Scan(ctx, strings.NewReader("file")); err == nil {
		t.Fatal("scanner timeout accepted as clean")
	}
}
