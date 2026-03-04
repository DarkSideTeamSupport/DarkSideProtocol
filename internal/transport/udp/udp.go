package udp

import (
	"context"
	"fmt"
	"net"
	"time"
)

type Server struct {
	conn *net.UDPConn
}

func Listen(addr string) (*Server, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, fmt.Errorf("resolve udp: %w", err)
	}
	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return nil, fmt.Errorf("listen udp: %w", err)
	}
	return &Server{conn: conn}, nil
}

func (s *Server) Serve(ctx context.Context, handler func(*net.UDPAddr, []byte)) error {
	defer s.conn.Close()
	buf := make([]byte, 2048)
	for {
		if err := s.conn.SetReadDeadline(time.Now().Add(1 * time.Second)); err != nil {
			return err
		}
		n, addr, err := s.conn.ReadFromUDP(buf)
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
		packet := make([]byte, n)
		copy(packet, buf[:n])
		handler(addr, packet)
	}
}

func (s *Server) WriteTo(addr *net.UDPAddr, payload []byte) error {
	_, err := s.conn.WriteToUDP(payload, addr)
	return err
}

type Client struct {
	conn *net.UDPConn
}

func Dial(addr string) (*Client, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, fmt.Errorf("resolve udp: %w", err)
	}
	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		return nil, fmt.Errorf("dial udp: %w", err)
	}
	return &Client{conn: conn}, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) Send(payload []byte) error {
	_, err := c.conn.Write(payload)
	return err
}

func (c *Client) Receive(timeout time.Duration) ([]byte, error) {
	if err := c.conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return nil, err
	}
	buf := make([]byte, 2048)
	n, err := c.conn.Read(buf)
	if err != nil {
		return nil, err
	}
	packet := make([]byte, n)
	copy(packet, buf[:n])
	return packet, nil
}
