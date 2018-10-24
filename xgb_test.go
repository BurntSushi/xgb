package xgb

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

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

type dNCEvent struct{}

func (_ dNCEvent) Bytes() []byte  { return nil }
func (_ dNCEvent) String() string { return "dummy X server event" }

type dNCError struct {
	seqId uint16
}

func (e dNCError) SequenceId() uint16 { return e.seqId }
func (_ dNCError) BadId() uint32      { return 0 }
func (_ dNCError) Error() string      { return "dummy X server error reply" }

func TestConnOnNonBlockingDummyXServer(t *testing.T) {
	timeout := time.Millisecond
	wantResponse := func(action func(*Conn) error, want, block error) func(*Conn) error {
		return func(s *Conn) error {
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
	NewErrorFuncs[255] = func(buf []byte) Error {
		return dNCError{Get16(buf[2:])}
	}
	NewEventFuncs[128&127] = func(buf []byte) Event {
		return dNCEvent{}
	}
	checkedReply := func(wantError bool) func(*Conn) error {
		request := "reply"
		if wantError {
			request = "error"
		}
		return func(c *Conn) error {
			cookie := c.NewCookie(true, true)
			c.NewRequest([]byte(request), cookie)
			_, err := cookie.Reply()
			if wantError && err == nil {
				return errors.New(fmt.Sprintf("checked request \"%v\" with reply resulted in nil error, want some error", request))
			}
			if !wantError && err != nil {
				return errors.New(fmt.Sprintf("checked request \"%v\" with reply resulted in error %v, want nil error", request, err))
			}
			return nil
		}
	}
	checkedNoreply := func(wantError bool) func(*Conn) error {
		request := "noreply"
		if wantError {
			request = "error"
		}
		return func(c *Conn) error {
			cookie := c.NewCookie(true, false)
			c.NewRequest([]byte(request), cookie)
			err := cookie.Check()
			if wantError && err == nil {
				return errors.New(fmt.Sprintf("checked request \"%v\" with no reply resulted in nil error, want some error", request))
			}
			if !wantError && err != nil {
				return errors.New(fmt.Sprintf("checked request \"%v\" with no reply resulted in error %v, want nil error", request, err))
			}
			return nil
		}
	}
	uncheckedReply := func(wantError bool) func(*Conn) error {
		request := "reply"
		if wantError {
			request = "error"
		}
		return func(c *Conn) error {
			cookie := c.NewCookie(false, true)
			c.NewRequest([]byte(request), cookie)
			_, err := cookie.Reply()
			if err != nil {
				return errors.New(fmt.Sprintf("unchecked request \"%v\" with reply resulted in %v, want nil", request, err))
			}
			return nil
		}
	}
	uncheckedNoreply := func(wantError bool) func(*Conn) error {
		request := "noreply"
		if wantError {
			request = "error"
		}
		return func(c *Conn) error {
			cookie := c.NewCookie(false, false)
			c.NewRequest([]byte(request), cookie)
			return nil
		}
	}
	event := func() func(*Conn) error {
		return func(c *Conn) error {
			_, err := c.conn.Write([]byte("event"))
			if err != nil {
				return errors.New(fmt.Sprintf("asked dummy server to send event, but resulted in error: %v\n", err))
			}
			return err
		}
	}
	waitEvent := func(wantError bool) func(*Conn) error {
		return func(c *Conn) error {
			_, err := c.WaitForEvent()
			if wantError && err == nil {
				return errors.New(fmt.Sprintf("wait for event resulted in nil error, want some error"))
			}
			if !wantError && err != nil {
				return errors.New(fmt.Sprintf("wait for event resulted in error %v, want nil error", err))
			}
			return nil
		}
	}

	testCases := []struct {
		description string
		actions     []func(*Conn) error
	}{
		{"close",
			[]func(*Conn) error{},
		},
		{"checked requests with reply",
			[]func(*Conn) error{
				checkedReply(false),
				checkedReply(true),
				checkedReply(false),
				checkedReply(true),
			},
		},
		{"checked requests no reply",
			[]func(*Conn) error{
				checkedNoreply(false),
				checkedNoreply(true),
				checkedNoreply(false),
				checkedNoreply(true),
			},
		},
		{"unchecked requests with reply",
			[]func(*Conn) error{
				uncheckedReply(false),
				uncheckedReply(true),
				waitEvent(true),
				uncheckedReply(false),
				event(),
				waitEvent(false),
			},
		},
		{"unchecked requests no reply",
			[]func(*Conn) error{
				uncheckedNoreply(false),
				uncheckedNoreply(true),
				waitEvent(true),
				uncheckedNoreply(false),
				event(),
				waitEvent(false),
			},
		},
		{"unexpected conn close",
			[]func(*Conn) error{
				func(c *Conn) error {
					c.conn.Close()
					if ev, err := c.WaitForEvent(); ev != nil || err != nil {
						return fmt.Errorf("after conn close WaitForEvent() = (%v, %v), want (nil, nil)", ev, err)
					}
					return nil
				},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			tclm := leaksMonitor("test case " + tc.description)
			defer tclm.checkTesting(t)

			seqId := uint16(1)
			incrementSequenceId := func() {
				// this has to be the same algorithm as in (*Conn).generateSeqIds
				if seqId == uint16((1<<16)-1) {
					seqId = 0
				} else {
					seqId++
				}
			}
			dummyXreplyer := func(request []byte) []byte {
				//fmt.Printf("dummyXreplyer got request: %s\n", string(request))
				res := make([]byte, 32)
				switch string(request) {
				case "event":
					res[0] = 128
					//fmt.Printf("dummyXreplyer sent response: %v\n", res)
					return res
				case "error":
					res[0] = 0   // error
					res[1] = 255 // error function
				default:
					res[0] = 1 // reply
				}
				Put16(res[2:], seqId) // sequence number
				incrementSequenceId()
				if string(request) == "noreply" {
					//fmt.Printf("dummyXreplyer no response sent\n")
					return nil
				}
				//fmt.Printf("dummyXreplyer sent response: %v\n", res)
				return res
			}

			sclm := leaksMonitor("after server close", tclm)
			defer sclm.checkTesting(t)
			s := newDummyNetConn("dummX", dummyXreplyer)
			defer s.Close()

			c, err := postNewConn(&Conn{conn: s})
			if err != nil {
				t.Errorf("connect to dummy server error: %v", err)
				return
			}

			rlm := leaksMonitor("after actions end")
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
	}
}
