package socksproxy

import (
	"io"
	"net"
	"time"
)

var _ net.Conn = (*ConnWrapper)(nil)

// ConnWrapper is used for socks5
type ConnWrapper struct {
	io.ReadWriteCloser
}

func (c ConnWrapper) LocalAddr() net.Addr {
	return nil
}

func (c ConnWrapper) RemoteAddr() net.Addr {
	// TODO implement
	//  we probably need this to return net.TCPAddr, to have correct address in proxy
	return nil
}

func (c ConnWrapper) SetDeadline(time.Time) error {
	return nil
}

func (c ConnWrapper) SetReadDeadline(time.Time) error {
	return nil
}

func (c ConnWrapper) SetWriteDeadline(time.Time) error {
	return nil
}
