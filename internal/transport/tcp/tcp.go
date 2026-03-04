package tcp

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"time"
)

type Server struct {
	ln net.Listener
}

func Listen(addr string) (*Server, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen tcp: %w", err)
	}
	return &Server{ln: ln}, nil
}

func (s *Server) Serve(ctx context.Context, handler func(net.Conn, []byte)) error {
	defer s.ln.Close()
	for {
		if dl, ok := s.ln.(*net.TCPListener); ok {
			_ = dl.SetDeadline(time.Now().Add(1 * time.Second))
		}
		conn, err := s.ln.Accept()
		if ne, ok := err.(net.Error); ok && ne.Timeout() {
			select {
			case <-ctx.Done():
				return nil
			default:
				continue
			}
		}
		if err != nil {
			return err
		}
		go func(c net.Conn) {
			defer c.Close()
			reader := bufio.NewReader(c)
			for {
				payload, err := readFrame(reader)
				if err != nil {
					if err != io.EOF {
						// keep silent in scaffold
					}
					return
				}
				handler(c, payload)
			}
		}(conn)
	}
}

func WriteFrame(w io.Writer, payload []byte) error {
	var hdr [2]byte
	if len(payload) > 65535 {
		return fmt.Errorf("frame too large: %d", len(payload))
	}
	binary.BigEndian.PutUint16(hdr[:], uint16(len(payload)))
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	_, err := w.Write(payload)
	return err
}

func readFrame(r *bufio.Reader) ([]byte, error) {
	var hdr [2]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return nil, err
	}
	size := binary.BigEndian.Uint16(hdr[:])
	if size == 0 {
		return nil, fmt.Errorf("empty frame")
	}
	buf := make([]byte, size)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

type Client struct {
	conn net.Conn
}

func Dial(addr string) (*Client, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("dial tcp: %w", err)
	}
	return &Client{conn: conn}, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) Send(payload []byte) error {
	return WriteFrame(c.conn, payload)
}

func (c *Client) Receive(timeout time.Duration) ([]byte, error) {
	if err := c.conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return nil, err
	}
	reader := bufio.NewReader(c.conn)
	return readFrame(reader)
}
