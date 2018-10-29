package xgb

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestConnOnNonBlockingDummyXServer(t *testing.T) {
	timeout := 10 * time.Millisecond
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
	checkClosed := func(c *Conn) error {
		select {
		case eoe, ok := <-c.eventChan:
			if ok {
				return fmt.Errorf("(*Conn).eventChan should be closed, but is not and returns %v", eoe)
			}
		case <-time.After(timeout):
			return fmt.Errorf("(*Conn).eventChan should be closed, but is not and was blocking for %v", timeout)
		}
		return nil
	}

	testCases := []struct {
		description string
		actions     []func(*Conn) error
	}{
		{"close",
			[]func(*Conn) error{},
		},
		{"double close",
			[]func(*Conn) error{
				func(c *Conn) error {
					c.Close()
					return nil
				},
			},
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
		{"close with pending requests",
			[]func(*Conn) error{
				func(c *Conn) error {
					c.conn.(*dNC).ReadLock()
					defer c.conn.(*dNC).ReadUnlock()
					c.NewRequest([]byte("reply"), c.NewCookie(false, true))
					c.Close()
					return nil
				},
				checkClosed,
			},
		},
		{"unexpected conn close",
			[]func(*Conn) error{
				func(c *Conn) error {
					c.conn.Close()
					if ev, err := c.WaitForEvent(); ev != nil || err != nil {
						return fmt.Errorf("WaitForEvent() = (%v, %v), want (nil, nil)", ev, err)
					}
					return nil
				},
				checkClosed,
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			sclm := leaksMonitor("after server close, before testcase exit")
			defer sclm.checkTesting(t)

			s := newDummyNetConn("dummyX", newDummyXServerReplier())
			defer s.Close()

			c, err := postNewConn(&Conn{conn: s})
			if err != nil {
				t.Errorf("connect to dummy server error: %v", err)
				return
			}

			defer leaksMonitor("after actions end", sclm).checkTesting(t)

			for _, action := range tc.actions {
				if err := action(c); err != nil {
					t.Error(err)
					break
				}
			}

			recovered := false
			func() {
				defer func() {
					if err := recover(); err != nil {
						t.Errorf("(*Conn).Close() panic recover: %v", err)
						recovered = true
					}
				}()

				c.Close()
			}()
			if !recovered {
				if err := checkClosed(c); err != nil {
					t.Error(err)
				}
			}

		})
	}
}
