package xgb

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestLeaks(t *testing.T) {
	lm := leaksMonitor("lm")
	if lgrs := lm.leakingGoroutines(); len(lgrs) != 0 {
		t.Errorf("leakingGoroutines returned %d leaking goroutines, want 0", len(lgrs))
	}

	done := make(chan struct{})
	wg := &sync.WaitGroup{}

	wg.Add(1)
	go func() {
		<-done
		wg.Done()
	}()

	if lgrs := lm.leakingGoroutines(); len(lgrs) != 1 {
		t.Errorf("leakingGoroutines returned %d leaking goroutines, want 1", len(lgrs))
	}

	wg.Add(1)
	go func() {
		<-done
		wg.Done()
	}()

	if lgrs := lm.leakingGoroutines(); len(lgrs) != 2 {
		t.Errorf("leakingGoroutines returned %d leaking goroutines, want 2", len(lgrs))
	}

	close(done)
	wg.Wait()

	if lgrs := lm.leakingGoroutines(); len(lgrs) != 0 {
		t.Errorf("leakingGoroutines returned %d leaking goroutines, want 0", len(lgrs))
	}

	lm.checkTesting(t)
	//TODO multiple leak monitors with report ignore tests
}

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

	timeout := 10 * time.Millisecond
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

func TestDummyXServerReplier(t *testing.T) {
	testCases := [][][2][]byte{
		{
			[2][]byte{[]byte("reply"), []byte{1, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
			[2][]byte{[]byte("eply"), []byte{1, 0, 2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
			[2][]byte{[]byte("ply"), []byte{1, 0, 3, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
			[2][]byte{[]byte("event"), []byte{128, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
			[2][]byte{[]byte("ly"), []byte{1, 0, 4, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
			[2][]byte{[]byte("y"), []byte{1, 0, 5, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
			[2][]byte{[]byte(""), []byte{1, 0, 6, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
			[2][]byte{[]byte("event"), []byte{128, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
			[2][]byte{[]byte("reply"), []byte{1, 0, 7, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
			[2][]byte{[]byte("error"), []byte{0, 255, 8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
			[2][]byte{[]byte("ply"), []byte{1, 0, 9, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
			[2][]byte{[]byte("event"), []byte{128, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
			[2][]byte{[]byte("ly"), []byte{1, 0, 10, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
			[2][]byte{[]byte("noreply"), nil},
			[2][]byte{[]byte("error"), []byte{0, 255, 12, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
			[2][]byte{[]byte("noreply"), nil},
			[2][]byte{[]byte(""), []byte{1, 0, 14, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
		},
	}

	for tci, tc := range testCases {
		replier := newDummyXServerReplier()
		for ai, ioPair := range tc {
			in, want := ioPair[0], ioPair[1]
			if out := replier(in); !bytes.Equal(out, want) {
				t.Errorf("testCase %d, action %d, replier(%s) = %v, want %v", tci, ai, string(in), out, want)
				break
			}
		}
	}
}
