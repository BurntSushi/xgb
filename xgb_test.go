package xgb

import (
	"net"
	"testing"
	"time"
)

type addr struct{}

func (_ addr) Network() string { return "" }
func (_ addr) String() string  { return "" }

type server struct{}

func (s *server) Write(b []byte) (int, error)        { return len(b), nil }
func (s *server) Read(b []byte) (int, error)         { return len(b), nil }
func (s *server) Close() error                       { return nil }
func (s *server) LocalAddr() Addr                    { return addr{} }
func (s *server) RemoteAddr() Addr                   { return addr{} }
func (s *server) SetDeadline(t time.Time) error      { return nil }
func (s *server) SetReadDeadline(t time.Time) error  { return nil }
func (s *server) SetWriteDeadline(t time.Time) error { return nil }

func dummyServer() net.Conn {
	return &server
}

func TestConnOpenClose(t *testing.T) {
}
