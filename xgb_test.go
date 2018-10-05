package xgb

import (
	"errors"
	"io"
	"net"
	"runtime"
	"testing"
	"time"
)

type addr struct{}

func (_ addr) Network() string { return "" }
func (_ addr) String() string  { return "" }

type server struct {
	control chan interface{}
	done    chan struct{}
}

func newServer() net.Conn {
	s := &server{
		make(chan interface{}),
		make(chan struct{}),
	}
	go func() {
		defer close(s.done)
		for {
			select {
			case ci := <-s.control:
				if ci == nil {
					return
				}
			}
		}
	}()
	return s
}

func (_ *server) errClosed() error {
	return errors.New("closed")
}
func (_ *server) errEOF() error {
	return io.EOF
}

func (s *server) Write(b []byte) (int, error) {
	select {
	case <-s.done:
	}
	return 0, s.errClosed()
}

func (s *server) Read(b []byte) (int, error) {
	select {
	case <-s.done:
	}
	return 0, s.errEOF()
}
func (s *server) Close() error {
	select {
	case s.control <- nil:
		<-s.done
		return nil
	case <-s.done:
		return s.errClosed()
	}
}
func (s *server) LocalAddr() net.Addr                { return addr{} }
func (s *server) RemoteAddr() net.Addr               { return addr{} }
func (s *server) SetDeadline(t time.Time) error      { return nil }
func (s *server) SetReadDeadline(t time.Time) error  { return nil }
func (s *server) SetWriteDeadline(t time.Time) error { return nil }

func TestConnOpenClose(t *testing.T) {
	ngrs := runtime.NumGoroutine()

	t.Logf("creating new dummy blocking server")
	s := newServer()
	defer func() {
		if err := s.Close(); err != nil {
			t.Errorf("server closing error: %v", err)
		}
	}()
	t.Logf("new server created: %v", s)

	leakTimeout := time.Second
	defer func() {
		if ngre := runtime.NumGoroutine(); ngrs != ngre {
			t.Logf("possible goroutine leakage, waiting %v", leakTimeout)
			time.Sleep(time.Second)
			if ngre := runtime.NumGoroutine(); ngrs != ngre {
				t.Errorf("goroutine leaks: start(%d) != end(%d)", ngrs, ngre)
			}
		}
	}()

	c, err := postNewConn(&Conn{conn: s})
	if err != nil {
		t.Fatalf("connect error: %v", err)
	}
	t.Logf("connection to server created: %v", c)

	closeErr := make(chan error, 1)
	closeTimeout := time.Second
	select {
	case closeErr <- func() error {
		t.Logf("closing connection to server")
		c.Close()
		t.Logf("connection to server closed")
		return nil
	}():
	case <-time.After(closeTimeout):
		t.Errorf("*Conn.Close() not responded for %v", closeTimeout)
	}

}
