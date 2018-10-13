package xgb

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

type addr struct {
	s string
}

func (_ addr) Network() string { return "dummy" }
func (a addr) String() string  { return a.s }

type errTimeout struct{ error }

func (_ errTimeout) Timeout() bool { return true }

var (
	dXErrNotImplemented = errors.New("command not implemented")
	dXErrClosed         = errors.New("server closed")
	dXErrWrite          = errors.New("server write failed")
	dXErrRead           = errors.New("server read failed")
	dXErrResponse       = errors.New("server response error")
)

type dXIoResult struct {
	n   int
	err error
}
type dXIo struct {
	b      []byte
	result chan dXIoResult
}

type dXCSendEvent struct{}
type dXCWriteLock struct{}
type dXCWriteUnlock struct{}
type dXCWriteError struct{}
type dXCWriteSuccess struct{ errorResponse bool }
type dXCReadLock struct{}
type dXCReadUnlock struct{}
type dXCReadError struct{}
type dXCReadSuccess struct{}

type dXEvent struct{}

func (_ dXEvent) Bytes() []byte  { return nil }
func (_ dXEvent) String() string { return "dummy X server event" }

type dXError struct {
	seqId uint16
}

func (e dXError) SequenceId() uint16 { return e.seqId }
func (_ dXError) BadId() uint32      { return 0 }
func (_ dXError) Error() string      { return "dummy X server error reply" }

// dummy X server implementing net.Conn interface,
type dX struct {
	addr    addr
	in, out chan dXIo
	control chan interface{}
	done    chan struct{}
}

// Results running dummy X server, satisfying net.Conn interface for test purposes.
// It is users responsibility to stop and clean up resources with (*dX).Close, if not needed anymore.
// By default, the read and write method are unlocked and will not result in error and read response will be a non error X reply (like with (*dX).WriteSuccess(false), (*dX).ReadSuccess()).
//TODO make (*dX).SetDeadline, (*dX).SetReadDeadline, (*dX).SetWriteDeadline work proprely.
func newDX(name string) *dX {
	s := &dX{
		addr{name},
		make(chan dXIo), make(chan dXIo),
		make(chan interface{}),
		make(chan struct{}),
	}

	seqId := uint16(1)
	incrementSequenceId := func() {
		// this has to be the same algorithm as in (*Conn).generateSeqIds
		if seqId == uint16((1<<16)-1) {
			seqId = 0
		} else {
			seqId++
		}
	}

	in, out := s.in, chan dXIo(nil)
	buf := &bytes.Buffer{}
	errorRead, errorWrite, errorResponse := false, false, false
	lockRead := false

	NewErrorFuncs[255] = func(buf []byte) Error {
		return dXError{Get16(buf[2:])}
	}
	NewEventFuncs[128&127] = func(buf []byte) Event {
		return dXEvent{}
	}

	go func() {
		defer close(s.done)
		for {
			select {
			case dxsio := <-in:
				if errorWrite {
					dxsio.result <- dXIoResult{0, dXErrWrite}
					break
				}

				response := make([]byte, 32)
				if errorResponse { // response will be error
					response[0] = 0   // error
					response[1] = 255 // error function
				} else { // response will by reply with no additional reply
					response[0] = 1 // reply
				}
				Put16(response[2:], seqId) // sequence number
				incrementSequenceId()

				buf.Write(response)
				dxsio.result <- dXIoResult{len(dxsio.b), nil}

				if !lockRead && out == nil {
					out = s.out
				}
			case dxsio := <-out:
				if errorRead {
					dxsio.result <- dXIoResult{0, dXErrRead}
					break
				}

				n, err := buf.Read(dxsio.b)
				dxsio.result <- dXIoResult{n, err}

				if buf.Len() == 0 {
					out = nil
				}
			case ci := <-s.control:
				if ci == nil {
					return
				}
				switch cs := ci.(type) {
				case dXCSendEvent:
					response := make([]byte, 32)
					response[0] = 128
					buf.Write(response)

					if !lockRead && out == nil {
						out = s.out
					}
				case dXCWriteLock:
					in = nil
				case dXCWriteUnlock:
					in = s.in
				case dXCWriteError:
					errorWrite = true
				case dXCWriteSuccess:
					errorWrite = false
					errorResponse = cs.errorResponse
				case dXCReadLock:
					out = nil
					lockRead = true
				case dXCReadUnlock:
					lockRead = false
					if buf.Len() > 0 && out == nil {
						out = s.out
					}
				case dXCReadError:
					errorRead = true
				case dXCReadSuccess:
					errorRead = false
				default:
				}
			}
		}
	}()
	return s
}

// Shuts down dummy X server. Every blocking or future method calls will do nothing and result in error.
// Result will be dXErrClosed if server was allready closed.
// Server can not be unclosed.
func (s *dX) Close() error {
	select {
	case s.control <- nil:
		<-s.done
		return nil
	case <-s.done:
	}
	return dXErrClosed
}

// Imitates write action to X server.
// If not locked by (*dX).WriteLock, it results in error or success.
//
// If Write errors, the second result parameter will be an error {dXErrWrite|dXErrClosed}, the resulting first parameter will be 0,
// no response will be generated and the internal sequence number will not be incremented.
//
// If Write succeedes, it results in (len(b), nil), the (*dX).Read will be unblocked (if not locked with (*dX).ReadLock),
// an [32]byte response will be written to buffer from which (*dX).Read reads,
// with sequence number and proper X response type (error or reply) and the internal sequence number will be increased.
//
// If server was closed previously, result will be (0, dXErrClosed).
func (s *dX) Write(b []byte) (int, error) {
	resChan := make(chan dXIoResult)
	//fmt.Printf("(*dX).Write: got write request: %v\n", b)
	select {
	case s.in <- dXIo{b, resChan}:
		//fmt.Printf("(*dX).Write: input channel has accepted request\n")
		res := <-resChan
		//fmt.Printf("(*dX).Write: got result: %v\n", res)
		return res.n, res.err
	case <-s.done:
	}
	//fmt.Printf("(*dX).Write: server was closed\n")
	return 0, dXErrClosed
}

// Imitates read action from X server.
// If locked by (*dX).ReadLock, read will block until unlocking with (*dX).ReadUnlock, or server closes.
//
// If not locked, Read will result in error, or block until internal read buffer is not empty, depending on internal state.
// The internal state can be modified via (*dX).ReadError, or (*dX).ReadSuccess
// ReadError makes it return (0, dXErrRead), with no changes to internal buffer or state.
// ReadSuccess makes it block until there are some write responses. After emtying the internal read buffer, all next Read requests will block untill another successful write requests.
//
// The resulting read success response type can be altered with (*dX).WriteSuccess method.
//
// If server was closed previously, result will be (0, io.EOF).
func (s *dX) Read(b []byte) (int, error) {
	resChan := make(chan dXIoResult)
	//fmt.Printf("(*dX).Read: got read request of length: %v\n", len(b))
	select {
	case s.out <- dXIo{b, resChan}:
		//fmt.Printf("(*dX).Read: output channel has accepted request\n")
		res := <-resChan
		//fmt.Printf("(*dX).Read: got result: %v\n", res)
		//fmt.Printf("(*dX).Read: result bytes: %v\n", b)
		return res.n, res.err
	case <-s.done:
		//fmt.Printf("(*dX).Read: server was closed\n")
	}
	return 0, io.EOF
}
func (s *dX) LocalAddr() net.Addr                { return s.addr }
func (s *dX) RemoteAddr() net.Addr               { return s.addr }
func (s *dX) SetDeadline(t time.Time) error      { return dXErrNotImplemented }
func (s *dX) SetReadDeadline(t time.Time) error  { return dXErrNotImplemented }
func (s *dX) SetWriteDeadline(t time.Time) error { return dXErrNotImplemented }

func (s *dX) Control(i interface{}) error {
	select {
	case s.control <- i:
		return nil
	case <-s.done:
	}
	return dXErrClosed
}

// Adds an Event into read buffer.
func (s *dX) SendEvent() error {
	return s.Control(dXCSendEvent{})
}

// Locks writing. All write requests will be blocked until write is unlocked with (*dX).WriteUnlock, or server closes.
func (s *dX) WriteLock() error {
	return s.Control(dXCWriteLock{})
}

// Unlocks writing. All blocked write requests until now will be accepted.
func (s *dX) WriteUnlock() error {
	return s.Control(dXCWriteUnlock{})
}

// Unlocks writing and makes (*dX).Write to result (0, dXErrWrite).
func (s *dX) WriteError() error {
	if err := s.WriteUnlock(); err != nil {
		return err
	}
	return s.Control(dXCWriteError{})
}

// Unlocks writing and makes (*dX).Write to result (len(b), nil), with a proper X reply.
// If errorResult is true, the response will be an X error response,
// else an normal reply. See (*dX).Write for details.
func (s *dX) WriteSuccess(errorResult bool) error {
	if err := s.WriteUnlock(); err != nil {
		return err
	}
	return s.Control(dXCWriteSuccess{errorResult})
}

// Locks reading. All read requests will be blocked until read is unlocked with (*dX).ReadUnlock, or server closes.
// (*dX).Read wil block even after successful write.
func (s *dX) ReadLock() error {
	return s.Control(dXCReadLock{})
}

// Unlocks reading. If there are any unresponded requests in reading buffer, read will be unblocked.
func (s *dX) ReadUnlock() error {
	return s.Control(dXCReadUnlock{})
}

// Unlocks read and makes every blocked and following (*dX).Read requests fail. See (*dX).Read for details.
func (s *dX) ReadError() error {
	if err := s.ReadUnlock(); err != nil {
		return err
	}
	return s.Control(dXCReadError{})
}

// Unlocks read and makes every blocked and following (*dX).Read requests be handled, if there are any in read buffer.
// See (*dX).Read for details.
func (s *dX) ReadSuccess() error {
	if err := s.ReadUnlock(); err != nil {
		return err
	}
	return s.Control(dXCReadSuccess{})
}

type goroutine struct {
	id    int
	name  string
	stack []byte
}

type leaks struct {
	name       string
	goroutines map[int]goroutine
	report     []*leaks
}

func leaksMonitor(name string, monitors ...*leaks) *leaks {
	return &leaks{
		name,
		leaks{}.collectGoroutines(),
		monitors,
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

func (l leaks) leakingGoroutines() []goroutine {
	goroutines := l.collectGoroutines()
	res := []goroutine{}
	for id, gr := range goroutines {
		if _, ok := l.goroutines[id]; ok {
			continue
		}
		res = append(res, gr)
	}
	return res
}
func (l leaks) checkTesting(t *testing.T) {
	if len(l.leakingGoroutines()) == 0 {
		return
	}
	leakTimeout := time.Second
	time.Sleep(leakTimeout)
	//t.Logf("possible goroutine leakage, waiting %v", leakTimeout)
	grs := l.leakingGoroutines()
	for _, gr := range grs {
		t.Errorf("%s: %s is leaking", l.name, gr.name)
		//t.Errorf("%s: %s is leaking\n%v", l.name, gr.name, string(gr.stack))
	}
	for _, rl := range l.report {
		rl.ignoreLeak(grs...)
	}
}
func (l *leaks) ignoreLeak(grs ...goroutine) {
	for _, gr := range grs {
		l.goroutines[gr.id] = gr
	}
}

func testDXCombinations(writeStates, readStates []string) []func() (*dX, error) {
	writeSetters := map[string]func(*dX) error{
		"lock":         (*dX).WriteLock,
		"error":        (*dX).WriteError,
		"successReply": func(s *dX) error { return s.WriteSuccess(false) },
		"successError": func(s *dX) error { return s.WriteSuccess(true) },
	}
	readSetters := map[string]func(*dX) error{
		"lock":    (*dX).ReadLock,
		"error":   (*dX).ReadError,
		"success": (*dX).ReadSuccess,
	}

	res := []func() (*dX, error){}
	for _, writeState := range writeStates {
		writeState, writeSetter := writeState, writeSetters[writeState]
		if writeSetter == nil {
			panic("unknown write state: " + writeState)
			continue
		}
		for _, readState := range readStates {
			readState, readSetter := readState, readSetters[readState]
			if readSetter == nil {
				panic("unknown read state: " + readState)
				continue
			}
			res = append(res, func() (*dX, error) {
				s := newDX("write=" + writeState + ",read=" + readState)

				if err := readSetter(s); err != nil {
					s.Close()
					return nil, errors.New("set read " + readState + " error: " + err.Error())
				}

				if err := writeSetter(s); err != nil {
					s.Close()
					return nil, errors.New("set write " + writeState + " error: " + err.Error())
				}

				return s, nil
			})
		}
	}
	return res
}

func TestDummyXServer(t *testing.T) {
	timeout := time.Millisecond
	wantResponse := func(action func(*dX) error, want, block error) func(*dX) error {
		return func(s *dX) error {
			actionResult := make(chan error)
			timedOut := make(chan struct{})
			go func() {
				err := action(s)
				select {
				case <-timedOut:
					if err != block {
						t.Errorf("after unblocking, action result=%v, want %v", err, block)
					}
				case actionResult <- err:
				}
			}()
			select {
			case err := <-actionResult:
				if err != want {
					return errors.New(fmt.Sprintf("action result=%v, want %v", err, want))
				}
			case <-time.After(timeout):
				close(timedOut)
				return errors.New(fmt.Sprintf("action did not respond for %v, result want %v", timeout, want))
			}
			return nil
		}
	}
	wantBlock := func(action func(*dX) error, unblock error) func(*dX) error {
		return func(s *dX) error {
			actionResult := make(chan error)
			timedOut := make(chan struct{})
			go func() {
				err := action(s)
				select {
				case <-timedOut:
					if err != unblock {
						t.Errorf("after unblocking, action result=%v, want %v", err, unblock)
					}
				case actionResult <- err:
				}
			}()
			select {
			case err := <-actionResult:
				return errors.New(fmt.Sprintf("action result=%v, want to be blocked", err))
			case <-time.After(timeout):
				close(timedOut)
			}
			return nil
		}
	}
	write := func() func(*dX) error {
		return func(s *dX) error {
			_, err := s.Write([]byte{1})
			return err
		}
	}
	read := func() func(*dX) error {
		return func(s *dX) error {
			b := make([]byte, 32)
			_, err := s.Read(b)
			return err
		}
	}
	readSuccess := func(seqId uint16, errorResponse bool) func(*dX) error {
		return func(s *dX) error {
			b := make([]byte, 32)
			_, err := s.Read(b)
			if err != nil {
				return err
			}
			if seqId != Get16(b[2:]) {
				return errors.New(fmt.Sprintf("got read sequence number %d, want %d", Get16(b[2:]), seqId))
			}
			b0, desc := 0, "error"
			if !errorResponse {
				b0, desc = 1, "valid"
			}
			if int(b[0]) != b0 {
				return errors.New(fmt.Sprintf("response is not an %s reply: %v", desc, b))
			}
			return nil
		}
	}

	testCases := []struct {
		description string
		servers     []func() (*dX, error)
		actions     []func(*dX) error // actions per server
	}{
		{"empty",
			[]func() (*dX, error){
				func() (*dX, error) { return newDX("server"), nil },
			},
			[]func(*dX) error{
				func(s *dX) error { return nil },
			},
		},
		{"close,close",
			testDXCombinations(
				[]string{"lock", "error", "successError", "successReply"},
				[]string{"lock", "error", "success"},
			),
			[]func(*dX) error{
				wantResponse((*dX).Close, nil, dXErrClosed),
				wantResponse((*dX).Close, dXErrClosed, dXErrClosed),
			},
		},
		{"write,close,write",
			testDXCombinations(
				[]string{"lock"},
				[]string{"lock", "error", "success"},
			),
			[]func(*dX) error{
				wantBlock(write(), dXErrClosed),
				wantResponse((*dX).Close, nil, dXErrClosed),
				wantResponse(write(), dXErrClosed, dXErrClosed),
			},
		},
		{"write,close,write",
			testDXCombinations(
				[]string{"error"},
				[]string{"lock", "error", "success"},
			),
			[]func(*dX) error{
				wantResponse(write(), dXErrWrite, dXErrClosed),
				wantResponse((*dX).Close, nil, dXErrClosed),
				wantResponse(write(), dXErrClosed, dXErrClosed),
			},
		},
		{"write,close,write",
			testDXCombinations(
				[]string{"successError", "successReply"},
				[]string{"lock", "error", "success"},
			),
			[]func(*dX) error{
				wantResponse(write(), nil, dXErrClosed),
				wantResponse((*dX).Close, nil, dXErrClosed),
				wantResponse(write(), dXErrClosed, dXErrClosed),
			},
		},
		{"read,close,read",
			testDXCombinations(
				[]string{"lock", "error", "successError", "successReply"},
				[]string{"lock", "error", "success"},
			),
			[]func(*dX) error{
				wantBlock(read(), io.EOF),
				wantResponse((*dX).Close, nil, dXErrClosed),
				wantResponse(read(), io.EOF, io.EOF),
			},
		},
		{"write,read",
			testDXCombinations(
				[]string{"lock"},
				[]string{"lock", "error", "success"},
			),
			[]func(*dX) error{
				wantBlock(write(), dXErrClosed),
				wantBlock(read(), io.EOF),
			},
		},
		{"write,read",
			testDXCombinations(
				[]string{"error"},
				[]string{"lock", "error", "success"},
			),
			[]func(*dX) error{
				wantResponse(write(), dXErrWrite, dXErrClosed),
				wantBlock(read(), io.EOF),
			},
		},
		{"write,read",
			testDXCombinations(
				[]string{"successError"},
				[]string{"lock"},
			),
			[]func(*dX) error{
				wantResponse(write(), nil, dXErrClosed),
				wantBlock(read(), io.EOF),
			},
		},
		{"write,read",
			testDXCombinations(
				[]string{"successError"},
				[]string{"error"},
			),
			[]func(*dX) error{
				wantResponse(write(), nil, dXErrClosed),
				wantResponse(read(), dXErrRead, io.EOF),
			},
		},
		{"write,read",
			testDXCombinations(
				[]string{"successError"},
				[]string{"success"},
			),
			[]func(*dX) error{
				wantResponse(write(), nil, dXErrClosed),
				wantResponse(readSuccess(1, true), nil, io.EOF),
			},
		},
		{"write,read",
			testDXCombinations(
				[]string{"successReply"},
				[]string{"success"},
			),
			[]func(*dX) error{
				wantResponse(write(), nil, dXErrClosed),
				wantResponse(readSuccess(1, false), nil, io.EOF),
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			defer leaksMonitor(tc.description).checkTesting(t)

			for _, server := range tc.servers {
				s, err := server()
				if err != nil {
					t.Error(err)
					continue
				}
				if s == nil {
					t.Error("nil server in testcase")
					continue
				}

				t.Run(s.LocalAddr().String(), func(t *testing.T) {
					defer leaksMonitor(s.LocalAddr().String()).checkTesting(t)
					for _, action := range tc.actions {
						if err := action(s); err != nil {
							t.Error(err)
							break
						}
					}
					s.Close()
				})
			}
		})
	}
}

func TestConnOnNonBlockingDummyXServer(t *testing.T) {
	timeout := time.Millisecond
	wantResponse := func(action func(*Conn) error, want, block error) func(*Conn) error {
		return func(c *Conn) error {
			actionResult := make(chan error)
			timedOut := make(chan struct{})
			go func() {
				err := action(c)
				select {
				case <-timedOut:
					if err != block {
						t.Errorf("after unblocking, action result=%v, want %v", err, block)
					}
				case actionResult <- err:
				}
			}()
			select {
			case err := <-actionResult:
				if err != want {
					return errors.New(fmt.Sprintf("action result=%v, want %v", err, want))
				}
			case <-time.After(timeout):
				close(timedOut)
				return errors.New(fmt.Sprintf("action did not respond for %v, result want %v", timeout, want))
			}
			return nil
		}
	}
	crequest := func(checked, reply bool) func(*Conn) error {
		return func(c *Conn) error {
			cookie := c.NewCookie(checked, reply)
			c.NewRequest([]byte("crequest"), cookie)
			_, err := cookie.Reply()
			return err
		}
	}

	testCases := []struct {
		description string
		servers     []func() (*dX, error)
		actions     []func(*Conn) error
	}{
		{"cclose",
			testDXCombinations(
				[]string{"successError", "successReply"},
				[]string{"success"},
			),
			[]func(*Conn) error{},
		},
		{"crequest",
			testDXCombinations([]string{"successError"}, []string{"success"}),
			[]func(*Conn) error{
				wantResponse(crequest(true, true), dXError{1}, io.ErrShortWrite),
			},
		},
		{"crequest",
			testDXCombinations([]string{"successReply"}, []string{"success"}),
			[]func(*Conn) error{
				wantResponse(crequest(true, true), nil, io.ErrShortWrite),
			},
		},
		// sometimes panic on unfixed branch - close of closed channel
		{"cclose",
			testDXCombinations([]string{"error"}, []string{"error", "success"}),
			[]func(*Conn) error{},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			tclm := leaksMonitor("test case " + tc.description)
			defer tclm.checkTesting(t)

			for _, server := range tc.servers {
				s, err := server()
				if err != nil {
					t.Error(err)
					continue
				}
				if s == nil {
					t.Error("nil *dX in testcase")
					continue
				}

				t.Run(s.LocalAddr().String(), func(t *testing.T) {
					sclm := leaksMonitor(s.LocalAddr().String()+" after sever close", tclm)
					defer sclm.checkTesting(t)

					c, err := postNewConn(&Conn{conn: s})
					if err != nil {
						t.Errorf("connect to dummy server error: %v", err)
						return
					}

					rlm := leaksMonitor(c.conn.LocalAddr().String() + " after actions end")
					for _, action := range tc.actions {
						if err := action(c); err != nil {
							t.Error(err)
							break
						}
					}
					c.Close()
					if err := wantResponse(
						func(c *Conn) error {
							if ev, err := c.WaitForEvent(); ev != nil || err != nil {
								return fmt.Errorf("after (*Conn).Close, (*Conn).WaitForEvent() = (%v,%v), want (nil,nil)", ev, err)
							}
							return nil
						},
						nil,
						io.ErrShortWrite,
					)(c); err != nil {
						t.Error(err)
					}
					rlm.checkTesting(t)
				})

				s.Close()
			}
		})
	}
}
