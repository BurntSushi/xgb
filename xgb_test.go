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
	"sync"
	"testing"
	"time"
)

type addr struct {
	s string
}

func (_ addr) Network() string { return "dummy" }
func (a addr) String() string  { return a.s }

var (
	serverErrEOF    = io.EOF
	serverErrClosed = errors.New("server closed")
	serverErrWrite  = errors.New("server write failed")
	serverErrRead   = errors.New("server read failed")
)

type serverBlocking struct {
	addr    addr
	control chan interface{}
	done    chan struct{}
}

func newServerBlocking(name string) *serverBlocking {
	s := &serverBlocking{
		addr{name},
		make(chan interface{}),
		make(chan struct{}),
	}
	runned := make(chan struct{})
	go func() {
		close(runned)
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
	<-runned
	return s
}

func (s *serverBlocking) Write(b []byte) (int, error) {
	select {
	case <-s.done:
	}
	return 0, serverErrClosed
}
func (s *serverBlocking) Read(b []byte) (int, error) {
	select {
	case <-s.done:
	}
	return 0, serverErrEOF
}
func (s *serverBlocking) Close() error {
	select {
	case s.control <- nil:
		<-s.done
		return nil
	case <-s.done:
		return serverErrClosed
	}
}
func (s *serverBlocking) LocalAddr() net.Addr                { return s.addr }
func (s *serverBlocking) RemoteAddr() net.Addr               { return s.addr }
func (s *serverBlocking) SetDeadline(t time.Time) error      { return nil }
func (s *serverBlocking) SetReadDeadline(t time.Time) error  { return nil }
func (s *serverBlocking) SetWriteDeadline(t time.Time) error { return nil }

type serverWriteErrorReadError struct {
	*serverBlocking
}

func newServerWriteErrorReadError(name string) *serverWriteErrorReadError {
	return &serverWriteErrorReadError{newServerBlocking(name)}
}
func (s *serverWriteErrorReadError) Write(b []byte) (int, error) {
	select {
	case <-s.done:
		return 0, serverErrClosed
	default:
	}
	return 0, serverErrWrite
}
func (s *serverWriteErrorReadError) Read(b []byte) (int, error) {
	select {
	case <-s.done:
		return 0, serverErrClosed
	default:
	}
	return 0, serverErrRead
}

type serverWriteErrorReadBlocking struct {
	*serverBlocking
}

func newServerWriteErrorReadBlocking(name string) *serverWriteErrorReadBlocking {
	return &serverWriteErrorReadBlocking{newServerBlocking(name)}
}
func (s *serverWriteErrorReadBlocking) Write(b []byte) (int, error) {
	select {
	case <-s.done:
		return 0, serverErrClosed
	default:
	}
	return 0, serverErrWrite
}

type serverWriteSuccessReadBlocking struct {
	*serverBlocking
}

func newServerWriteSuccessReadBlocking(name string) *serverWriteSuccessReadBlocking {
	return &serverWriteSuccessReadBlocking{newServerBlocking(name)}
}
func (s *serverWriteSuccessReadBlocking) Write(b []byte) (int, error) {
	select {
	case <-s.done:
		return 0, serverErrClosed
	default:
	}
	return len(b), nil
}

type serverWriteSuccessReadErrorAfterWrite struct {
	*serverBlocking
	out chan struct{}
	wg  *sync.WaitGroup
}

func newServerWriteSuccessReadErrorAfterWrite(name string) *serverWriteSuccessReadErrorAfterWrite {
	return &serverWriteSuccessReadErrorAfterWrite{
		newServerBlocking(name),
		make(chan struct{}),
		&sync.WaitGroup{},
	}
}
func (s *serverWriteSuccessReadErrorAfterWrite) Write(b []byte) (int, error) {
	select {
	case <-s.done:
		return 0, serverErrClosed
	default:
	}
	s.wg.Add(1)
	go func() {
		select {
		case s.out <- struct{}{}:
		case <-s.done:
		}
		s.wg.Done()
	}()
	return len(b), nil
}
func (s *serverWriteSuccessReadErrorAfterWrite) Read(b []byte) (int, error) {
	select {
	case <-s.done:
		return 0, serverErrClosed
	default:
	}
	select {
	case <-s.out:
		return 0, serverErrRead
	case <-s.done:
	}
	return 0, serverErrClosed
}
func (s *serverWriteSuccessReadErrorAfterWrite) Close() error {
	if err := s.serverBlocking.Close(); err != nil {
		return err
	}
	s.wg.Wait()
	return nil
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

// ispired by https://golang.org/src/runtime/debug/stack.go?s=587:606#L21
// stack returns a formatted stack trace of all goroutines.
// It calls runtime.Stack with a large enough buffer to capture the entire trace.
func (_ leaks) stack() []byte {
	buf := make([]byte, 1024)
	for {
		n := runtime.Stack(buf, true)
		if n < len(buf) {
			return buf[:n]
		}
		buf = make([]byte, 2*len(buf))
	}
}

func (l leaks) collectGoroutines() map[int]goroutine {
	res := make(map[int]goroutine)
	stacks := bytes.Split(l.stack(), []byte{'\n', '\n'})

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
		name := strings.TrimSpace(string(lines[1]))

		//filter out our stack routine
		if strings.Contains(name, "xgb.leaks.stack") {
			continue
		}

		res[id] = goroutine{id, name, st}
	}
	return res
}

func (l leaks) checkTesting(t *testing.T) {
	{
		goroutines := l.collectGoroutines()
		if len(l.goroutines) >= len(goroutines) {
			return
		}
	}
	leakTimeout := time.Second
	time.Sleep(leakTimeout)
	//t.Logf("possible goroutine leakage, waiting %v", leakTimeout)
	goroutines := l.collectGoroutines()
	if len(l.goroutines) >= len(goroutines) {
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

func TestDummyServersRunClose(t *testing.T) {

	testCases := []struct {
		name             string
		serverConstuctor func(string) net.Conn
	}{
		{"write blocking,read blocking server", func(n string) net.Conn { return newServerBlocking(n) }},
		{"write error,read error server", func(n string) net.Conn { return newServerWriteErrorReadError(n) }},
		{"write error,read blocking server", func(n string) net.Conn { return newServerWriteErrorReadBlocking(n) }},
		{"write success,read blocking server", func(n string) net.Conn { return newServerWriteSuccessReadBlocking(n) }},
		{"write success,read error afer write server", func(n string) net.Conn { return newServerWriteSuccessReadErrorAfterWrite(n) }},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			defer leaksMonitor().checkTesting(t)

			serverConn := tc.serverConstuctor(tc.name)

			{
				closeErr := make(chan error)
				go func() {
					closeErr <- serverConn.Close()
					close(closeErr)
				}()
				closeTimeout := time.Second
				select {
				case err := <-closeErr:
					want := error(nil)
					if err != want {
						t.Errorf("(net.Conn).Close()=%v, want %v", err, want)
					}
				case <-time.After(closeTimeout):
					t.Errorf("*Conn.Close() not responded for %v", closeTimeout)
				}
			}
			{
				closeErr := make(chan error)
				go func() {
					closeErr <- serverConn.Close()
					close(closeErr)
				}()
				closeTimeout := time.Second
				select {
				case err := <-closeErr:
					want := serverErrClosed
					if err != want {
						t.Errorf("(net.Conn).Close()=%v, want %v", err, want)
					}
				case <-time.After(closeTimeout):
					t.Errorf("*Conn.Close() not responded for %v", closeTimeout)
				}
			}
		})
	}
}

func TestConnOpenClose(t *testing.T) {

	testCases := []struct {
		name             string
		serverConstuctor func(string) net.Conn
	}{
		//{"blocking server", func(n string) net.Conn { return newServerBlocking(n) }}, // i'm not ready to handle this yet
		{"write error,read error server", func(n string) net.Conn { return newServerWriteErrorReadError(n) }},
		{"write error,read blocking server", func(n string) net.Conn { return newServerWriteErrorReadBlocking(n) }},
		{"write success,read blocking server", func(n string) net.Conn { return newServerWriteSuccessReadBlocking(n) }},
		{"write success,read error afer write server", func(n string) net.Conn { return newServerWriteSuccessReadErrorAfterWrite(n) }},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			serverConn := tc.serverConstuctor(tc.name)
			defer serverConn.Close()

			defer leaksMonitor().checkTesting(t)

			c, err := postNewConn(&Conn{conn: serverConn})
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
		})
	}

}
