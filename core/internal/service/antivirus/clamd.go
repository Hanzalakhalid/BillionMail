package antivirus

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

var ErrInfected = errors.New("malware detected")

type Scanner interface {
	Scan(context.Context, io.Reader) error
	Health(context.Context) error
}
type Clamd struct {
	Address string
	Timeout time.Duration
}

func (c Clamd) connect(ctx context.Context) (net.Conn, error) {
	if c.Address == "" {
		return nil, errors.New("antivirus address is required")
	}
	conn, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "tcp", c.Address)
	if err != nil {
		return nil, fmt.Errorf("antivirus unavailable: %w", err)
	}
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	deadline := time.Now().Add(timeout)
	if limit, ok := ctx.Deadline(); ok && limit.Before(deadline) {
		deadline = limit
	}
	if err = conn.SetDeadline(deadline); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

func reply(reader io.Reader) (string, error) {
	result, err := bufio.NewReader(io.LimitReader(reader, 4096)).ReadString(0)
	if err != nil {
		return "", fmt.Errorf("incomplete antivirus response: %w", err)
	}
	return strings.TrimSuffix(result, "\x00"), nil
}

func classify(response string) error {
	if response == "stream: OK" {
		return nil
	}
	if strings.HasPrefix(response, "stream: ") && strings.HasSuffix(response, " FOUND") {
		return ErrInfected
	}
	return errors.New("antivirus did not confirm a clean scan")
}

func (c Clamd) Scan(ctx context.Context, body io.Reader) error {
	if body == nil {
		return errors.New("antivirus stream is required")
	}
	conn, err := c.connect(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	// Drain a possible early error while sending; clamd may stop consuming data
	// after a configured size limit, so a write-only client can otherwise stall.
	type response struct {
		value string
		err   error
	}
	responses := make(chan response, 1)
	go func() { value, err := reply(conn); responses <- response{value, err}; conn.Close() }()
	writer := bufio.NewWriter(conn)
	if _, err = writer.WriteString("zINSTREAM\x00"); err == nil {
		buffer := make([]byte, 32*1024)
		for {
			n, readErr := body.Read(buffer)
			if n > 0 {
				if err = binary.Write(writer, binary.BigEndian, uint32(n)); err != nil {
					break
				}
				if _, err = writer.Write(buffer[:n]); err != nil {
					break
				}
			}
			if readErr != nil {
				if readErr != io.EOF {
					err = readErr
				}
				break
			}
		}
	}
	if err == nil {
		err = binary.Write(writer, binary.BigEndian, uint32(0))
	}
	if err == nil {
		err = writer.Flush()
	}
	if err != nil {
		conn.Close()
		answer := <-responses
		if answer.err == nil && errors.Is(classify(answer.value), ErrInfected) {
			return ErrInfected
		}
		return fmt.Errorf("antivirus stream failed: %w", err)
	}
	answer := <-responses
	if answer.err != nil {
		return answer.err
	}
	return classify(answer.value)
}

func (c Clamd) Health(ctx context.Context) error {
	conn, err := c.connect(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	if _, err = io.WriteString(conn, "zVERSIONCOMMANDS\x00"); err != nil {
		return err
	}
	response, err := reply(conn)
	if err != nil {
		return err
	}
	if !strings.Contains(response, "INSTREAM") {
		return errors.New("antivirus does not support streaming scans")
	}
	return nil
}
