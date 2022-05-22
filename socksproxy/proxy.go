package socksproxy

import (
	"errors"
	"io"
	"net"
	"sync"

	"github.com/haxii/socks5"
	log "github.com/sirupsen/logrus"
)

type Server struct {
	socks     *socks5.Server
	conns     map[net.Conn]struct{}
	connsLock sync.Mutex
}

func NewServer() *Server {
	conf := &socks5.Config{
		// we don't bind address for server
		BindIP:   net.IPv4(127, 0, 0, 1),
		BindPort: 8000,
	}
	server, err := socks5.New(conf)
	if err != nil {
		panic(err)
	}

	return &Server{
		socks: server,
		conns: map[net.Conn]struct{}{},
	}
}

func (s *Server) ServeConn(ioConn io.ReadWriteCloser) {
	conn := ConnWrapper{ReadWriteCloser: ioConn}
	s.connsLock.Lock()
	s.conns[conn] = struct{}{}
	s.connsLock.Unlock()

	go func() {
		err := s.socks.ServeConn(conn)
		if err != nil {
			log.Warnf("proxy: server: ServeConn: %v", err)
		}
		s.connsLock.Lock()
		delete(s.conns, conn)
		s.connsLock.Unlock()
	}()
}

func (s *Server) Close() error {
	s.connsLock.Lock()
	defer s.connsLock.Unlock()

	for conn := range s.conns {
		err := conn.Close()
		if err != nil {
			log.Warnf("proxy: server: close conn: %v", err)
		}
	}

	return nil
}

type Client struct {
	listener net.Listener
	connsCh  chan net.Conn
}

func NewClient(listenAddr string) (*Client, error) {
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return nil, err
	}

	cli := Client{
		listener: listener,
		connsCh:  make(chan net.Conn, 1),
	}
	go func() {
		err := cli.serve()
		if err != nil {
			log.Warnf("proxy: client: serve listener: %v", err)
		}
	}()

	return &cli, nil
}

func (c *Client) Close() error {
	return c.listener.Close()
}

func (c *Client) ConnsChan() chan net.Conn {
	return c.connsCh
}

func (c *Client) serve() error {
	for {
		conn, err := c.listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}

		c.connsCh <- conn
	}
}
