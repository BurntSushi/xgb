package xgb

import (
	"errors"
	"fmt"
	"io"
	"reflect"
	"testing"
	"time"
)

func TestDummyNetConn(t *testing.T) {
	ioStatesPairGenerator := func(writeStates, readStates []string) []func() (*dNC, error) {
		writeSetters := map[string]func(*dNC) error{
			"lock":    (*dNC).WriteLock,
			"error":   (*dNC).WriteError,
			"success": (*dNC).WriteSuccess,
		}
		readSetters := map[string]func(*dNC) error{
			"lock":    (*dNC).ReadLock,
			"error":   (*dNC).ReadError,
			"success": (*dNC).ReadSuccess,
		}

		res := []func() (*dNC, error){}
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
				res = append(res, func() (*dNC, error) {

					// loopback server
					s := newDummyNetConn("w:"+writeState+";r:"+readState, func(b []byte) []byte { return b })

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

	timeout := time.Millisecond
	wantResponse := func(action func(*dNC) error, want, block error) func(*dNC) error {
		return func(s *dNC) error {
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
	wantBlock := func(action func(*dNC) error, unblock error) func(*dNC) error {
		return func(s *dNC) error {
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
	write := func(b string) func(*dNC) error {
		return func(s *dNC) error {
			n, err := s.Write([]byte(b))
			if err == nil && n != len(b) {
				return errors.New("Write returned nil error, but not everything was written")
			}
			return err
		}
	}
	read := func(b string) func(*dNC) error {
		return func(s *dNC) error {
			r := make([]byte, len(b))
			n, err := s.Read(r)
			if err == nil {
				if n != len(b) {
					return errors.New("Read returned nil error, but not everything was read")
				}
				if !reflect.DeepEqual(r, []byte(b)) {
					return errors.New("Read=\"" + string(r) + "\", want \"" + string(b) + "\"")
				}
			}
			return err
		}
	}

	testCases := []struct {
		description string
		servers     []func() (*dNC, error)
		actions     []func(*dNC) error // actions per server
	}{
		{"close,close",
			ioStatesPairGenerator(
				[]string{"lock", "error", "success"},
				[]string{"lock", "error", "success"},
			),
			[]func(*dNC) error{
				wantResponse((*dNC).Close, nil, dNCErrClosed),
				wantResponse((*dNC).Close, dNCErrClosed, dNCErrClosed),
			},
		},
		{"write,close,write",
			ioStatesPairGenerator(
				[]string{"lock"},
				[]string{"lock", "error", "success"},
			),
			[]func(*dNC) error{
				wantBlock(write(""), dNCErrClosed),
				wantResponse((*dNC).Close, nil, dNCErrClosed),
				wantResponse(write(""), dNCErrClosed, dNCErrClosed),
			},
		},
		{"write,close,write",
			ioStatesPairGenerator(
				[]string{"error"},
				[]string{"lock", "error", "success"},
			),
			[]func(*dNC) error{
				wantResponse(write(""), dNCErrWrite, dNCErrClosed),
				wantResponse((*dNC).Close, nil, dNCErrClosed),
				wantResponse(write(""), dNCErrClosed, dNCErrClosed),
			},
		},
		{"write,close,write",
			ioStatesPairGenerator(
				[]string{"success"},
				[]string{"lock", "error", "success"},
			),
			[]func(*dNC) error{
				wantResponse(write(""), nil, dNCErrClosed),
				wantResponse((*dNC).Close, nil, dNCErrClosed),
				wantResponse(write(""), dNCErrClosed, dNCErrClosed),
			},
		},
		{"read,close,read",
			ioStatesPairGenerator(
				[]string{"lock", "error", "success"},
				[]string{"lock", "error", "success"},
			),
			[]func(*dNC) error{
				wantBlock(read(""), io.EOF),
				wantResponse((*dNC).Close, nil, dNCErrClosed),
				wantResponse(read(""), io.EOF, io.EOF),
			},
		},
		{"write,read",
			ioStatesPairGenerator(
				[]string{"lock"},
				[]string{"lock", "error", "success"},
			),
			[]func(*dNC) error{
				wantBlock(write("1"), dNCErrClosed),
				wantBlock(read("1"), io.EOF),
			},
		},
		{"write,read",
			ioStatesPairGenerator(
				[]string{"error"},
				[]string{"lock", "error", "success"},
			),
			[]func(*dNC) error{
				wantResponse(write("1"), dNCErrWrite, dNCErrClosed),
				wantBlock(read("1"), io.EOF),
			},
		},
		{"write,read",
			ioStatesPairGenerator(
				[]string{"success"},
				[]string{"lock"},
			),
			[]func(*dNC) error{
				wantResponse(write("1"), nil, dNCErrClosed),
				wantBlock(read("1"), io.EOF),
			},
		},
		{"write,read",
			ioStatesPairGenerator(
				[]string{"success"},
				[]string{"error"},
			),
			[]func(*dNC) error{
				wantResponse(write("1"), nil, dNCErrClosed),
				wantResponse(read("1"), dNCErrRead, io.EOF),
			},
		},
		{"write,read",
			ioStatesPairGenerator(
				[]string{"success"},
				[]string{"success"},
			),
			[]func(*dNC) error{
				wantResponse(write("1"), nil, dNCErrClosed),
				wantResponse(read("1"), nil, io.EOF),
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
