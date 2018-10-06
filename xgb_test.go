package xgb

import (
	"bytes"
	"errors"
	"io"
	"net"
	"regexp"
	"runtime"
	"strconv"
	"strings"
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

// ispired by https://golang.org/src/runtime/debug/stack.go?s=587:606#L21
// stack returns a formatted stack trace of all goroutines.
// It calls runtime.Stack with a large enough buffer to capture the entire trace.
func stack() []byte {
	buf := make([]byte, 1024)
	for {
		n := runtime.Stack(buf, true)
		if n < len(buf) {
			return buf[:n]
		}
		buf = make([]byte, 2*len(buf))
	}
}

type goroutine struct {
	id    int
	name  string
	stack []byte
}

type leaks struct {
	goroutines map[int]goroutine
}

func leaksMonitor() leaks {
	return leaks{
		leaks{}.collectGoroutines(),
	}
}

func (_ leaks) collectGoroutines() map[int]goroutine {
	res := make(map[int]goroutine)
	stacks := bytes.Split(stack(), []byte{'\n', '\n'})

	regexpId := regexp.MustCompile(`^\s*goroutine\s*(\d+)`)
	for _, st := range stacks {
		lines := bytes.Split(st, []byte{'\n'})
		if len(lines) < 2 {
			panic("routine stach has less tnan two lines: " + string(st))
		}

		idMatches := regexpId.FindSubmatch(lines[0])
		if len(idMatches) < 2 {
			panic("no id found in goroutine stack's first line: " + string(lines[0]))
		}
		id, err := strconv.Atoi(string(idMatches[1]))
		if err != nil {
			panic("converting goroutine id to number error: " + err.Error())
		}
		if _, ok := res[id]; ok {
			panic("2 goroutines with same id: " + strconv.Itoa(id))
		}

		//TODO filter out test routines, stack routine
		res[id] = goroutine{id, strings.TrimSpace(string(lines[1])), st}
	}
	return res
}

func (l leaks) checkTesting(t *testing.T) {
	{
		goroutines := l.collectGoroutines()
		if len(l.goroutines) == len(goroutines) {
			return
		}
	}
	leakTimeout := time.Second
	t.Logf("possible goroutine leakage, waiting %v", leakTimeout)
	goroutines := l.collectGoroutines()
	if len(l.goroutines) == len(goroutines) {
		return
	}
	t.Errorf("%d goroutine leaks: start(%d) != end(%d)", len(goroutines)-len(l.goroutines), len(l.goroutines), len(goroutines))
	for id, gr := range goroutines {
		if _, ok := l.goroutines[id]; ok {
			continue
		}
		t.Log(gr.name, "\n", string(gr.stack))
	}
}

func TestConnOpenClose(t *testing.T) {

	//t.Logf("creating new dummy blocking server")
	s := newServer()
	defer func() {
		if err := s.Close(); err != nil {
			t.Errorf("server closing error: %v", err)
		}
	}()
	//t.Logf("new server created: %v", s)

	defer leaksMonitor().checkTesting(t)

	c, err := postNewConn(&Conn{conn: s})
	if err != nil {
		t.Fatalf("connect error: %v", err)
	}
	//t.Logf("connection to server created: %v", c)

	closeErr := make(chan struct{})
	go func() {
		//t.Logf("closing connection to server")
		c.Close()
		close(closeErr)
	}()
	closeTimeout := time.Second
	select {
	case <-closeErr:
		//t.Logf("connection to server closed")
	case <-time.After(closeTimeout):
		t.Errorf("*Conn.Close() not responded for %v", closeTimeout)
	}

}
