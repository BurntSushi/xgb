// Copyright 2009 The XGB Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The XGB package implements the X11 core protocol.
// It is based on XCB: http://xcb.freedesktop.org/
package xgb

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
)

const (
	readBuffer  = 100
	writeBuffer = 100
)

// A Conn represents a connection to an X server.
type Conn struct {
	host          string
	conn          net.Conn
	nextCookie    uint16
	cookies       map[uint16]*Cookie
	events        queue
	err           error
	display       string
	defaultScreen int
	scratch       [32]byte
	Setup         SetupInfo
	extensions    map[string]byte

	requestChan       chan *Request
	requestCookieChan chan *Cookie
	replyChan         chan bool
	eventChan         chan bool
	errorChan         chan bool

	xidChan chan xid
	newIdLock   sync.Mutex
	writeLock   sync.Mutex
	dequeueLock sync.Mutex
	cookieLock  sync.Mutex
	extLock     sync.Mutex
}

// NewConn creates a new connection instance. It initializes locks, data
// structures, and performs the initial handshake. (The code for the handshake
// has been relegated to conn.go.)
func NewConn() (*Conn, error) {
	return NewConnDisplay("")
}

// NewConnDisplay is just like NewConn, but allows a specific DISPLAY
// string to be used.
// If 'display' is empty it will be taken from os.Getenv("DISPLAY").
//
// Examples:
//	NewConn(":1") -> net.Dial("unix", "", "/tmp/.X11-unix/X1")
//	NewConn("/tmp/launch-123/:0") -> net.Dial("unix", "", "/tmp/launch-123/:0")
//	NewConn("hostname:2.1") -> net.Dial("tcp", "", "hostname:6002")
//	NewConn("tcp/hostname:1.0") -> net.Dial("tcp", "", "hostname:6001")
func NewConnDisplay(display string) (*Conn, error) {
	conn := &Conn{}

	// First connect. This reads authority, checks DISPLAY environment
	// variable, and loads the initial Setup info.
	err := conn.connect(display)
	if err != nil {
		return nil, err
	}

	conn.xidChan = make(chan xid, 5)
	go conn.generateXids()

	conn.nextCookie = 1
	conn.cookies = make(map[uint16]*Cookie)
	conn.events = queue{make([][]byte, 100), 0, 0}
	conn.extensions = make(map[string]byte)

	conn.newReadChannels()
	conn.newRequestChannels()

	return conn, nil
}

// Close closes the connection to the X server.
func (c *Conn) Close() {
	c.conn.Close()
}

// Id is used for all X identifiers, such as windows, pixmaps, and GCs.
type Id uint32

// Request is used to abstract the difference between a request
// that expects a reply and a request that doesn't expect a reply.
type Request struct {
	buf        []byte
	cookieChan chan *Cookie
}

func newRequest(buf []byte, needsReply bool) *Request {
	req := &Request{
		buf:        buf,
		cookieChan: nil,
	}
	if needsReply {
		req.cookieChan = make(chan *Cookie)
	}
	return req
}

// Cookies are the sequence numbers used to pair replies up with their requests
type Cookie struct {
	id        uint16
	replyChan chan []byte
	errorChan chan error
}

func newCookie(id uint16) *Cookie {
	return &Cookie{
		id:        id,
		replyChan: make(chan []byte, 1),
		errorChan: make(chan error, 1),
	}
}

// Event is an interface that can contain any of the events returned by the
// server. Use a type assertion switch to extract the Event structs.
type Event interface {
	ImplementsEvent()
}

// newEventFuncs is a map from event numbers to functions that create
// the corresponding event.
var newEventFuncs = map[int]func(buf []byte) Event{}

// Error is an interface that can contain any of the errors returned by
// the server. Use a type assertion switch to extract the Error structs.
type Error interface {
	ImplementsError()
	SequenceId() uint16
	BadId() Id
	Error() string
}

// newErrorFuncs is a map from error numbers to functions that create
// the corresponding error.
var newErrorFuncs = map[int]func(buf []byte) Error{}

// NewID generates a new unused ID for use with requests like CreateWindow.
// If no new ids can be generated, the id returned is 0 and error is non-nil.
func (c *Conn) NewId() (Id, error) {
	xid := <-c.xidChan
	if xid.err != nil {
		return 0, xid.err
	}
	return xid.id, nil
}

// xid encapsulates a resource identifier being sent over the Conn.xidChan
// channel. If no new resource id can be generated, id is set to -1 and a
// non-nil error is set in xid.err.
type xid struct {
	id Id
	err error
}

// generateXids sends new Ids down the channel for NewId to use.
// This needs to be updated to use the XC Misc extension once we run out of
// new ids.
func (conn *Conn) generateXids() {
	inc := conn.Setup.ResourceIdMask & -conn.Setup.ResourceIdMask
	max := conn.Setup.ResourceIdMask
	last := uint32(0)
	for {
		// TODO: Use the XC Misc extension to look for released ids.
		if last > 0 && last >= max - inc + 1 {
			conn.xidChan <- xid{
				id: Id(0),
				err: errors.New("There are no more available resource" +
					"identifiers."),
			}
		}

		last += inc
		conn.xidChan <- xid{
			id: Id(last | conn.Setup.ResourceIdBase),
			err: nil,
		}
	}
}

// RegisterExtension adds the respective extension's major op code to
// the extensions map.
func (c *Conn) RegisterExtension(name string) error {
	nameUpper := strings.ToUpper(name)
	reply, err := c.QueryExtension(uint16(len(nameUpper)), nameUpper)

	switch {
	case err != nil:
		return err
	case !reply.Present:
		return errors.New(fmt.Sprintf("No extension named '%s' is present.",
			nameUpper))
	}

	c.extLock.Lock()
	c.extensions[nameUpper] = reply.MajorOpcode
	c.extLock.Unlock()

	return nil
}

// A simple queue used to stow away events.
type queue struct {
	data [][]byte
	a, b int
}

func (q *queue) queue(item []byte) {
	if q.b == len(q.data) {
		if q.a > 0 {
			copy(q.data, q.data[q.a:q.b])
			q.a, q.b = 0, q.b-q.a
		} else {
			newData := make([][]byte, (len(q.data)*3)/2)
			copy(newData, q.data)
			q.data = newData
		}
	}
	q.data[q.b] = item
	q.b++
}

func (q *queue) dequeue(c *Conn) []byte {
	c.dequeueLock.Lock()
	defer c.dequeueLock.Unlock()

	if q.a < q.b {
		item := q.data[q.a]
		q.a++
		return item
	}
	return nil
}

// newWriteChan creates the channel required for writing to the net.Conn.
func (c *Conn) newRequestChannels() {
	c.requestChan = make(chan *Request, writeBuffer)
	c.requestCookieChan = make(chan *Cookie, 1)

	go func() {
		for request := range c.requestChan {
			cookieNum := c.nextCookie
			c.nextCookie++

			if request.cookieChan != nil {
				cookie := newCookie(cookieNum)
				c.cookies[cookieNum] = cookie
				request.cookieChan <- cookie
			}
			if _, err := c.conn.Write(request.buf); err != nil {
				fmt.Fprintf(os.Stderr, "x protocol write error: %s\n", err)
				close(c.requestChan)
				return
			}
		}
	}()
}

// request is a buffered write to net.Conn.
func (c *Conn) request(buf []byte, needsReply bool) *Cookie {
	req := newRequest(buf, needsReply)
	c.requestChan <- req

	if req.cookieChan != nil {
		cookie := <-req.cookieChan
		close(req.cookieChan)
		return cookie
	}
	return nil
}

func (c *Conn) sendRequest(needsReply bool, bufs ...[]byte) *Cookie {
	if len(bufs) == 1 {
		return c.request(bufs[0], needsReply)
	}

	total := make([]byte, 0)
	for _, buf := range bufs {
		total = append(total, buf...)
	}
	return c.request(total, needsReply)
}

func (c *Conn) newReadChannels() {
	c.eventChan = make(chan bool, readBuffer)

	onError := func() {
		panic("read error")
	}

	go func() {
		for {
			buf := make([]byte, 32)
			if _, err := io.ReadFull(c.conn, buf); err != nil {
				fmt.Fprintf(os.Stderr, "x protocol read error: %s\n", err)
				onError()
				return
			}

			switch buf[0] {
			case 0:
				// err := &Error{ 
					// Detail: buf[1], 
					// Cookie: uint16(get16(buf[2:])), 
					// Id:     Id(get32(buf[4:])), 
					// Minor:  get16(buf[8:]), 
					// Major:  buf[10], 
				// } 
				err := newErrorFuncs[int(buf[1])](buf)
				if cookie, ok := c.cookies[err.SequenceId()]; ok {
					cookie.errorChan <- err
				} else {
					fmt.Fprintf(os.Stderr, "x protocol error: %s\n", err)
				}
			case 1:
				seq := uint16(Get16(buf[2:]))
				if _, ok := c.cookies[seq]; !ok {
					continue
				}

				size := Get32(buf[4:])
				if size > 0 {
					bigbuf := make([]byte, 32+size*4, 32+size*4)
					copy(bigbuf[0:32], buf)
					if _, err := io.ReadFull(c.conn, bigbuf[32:]); err != nil {
						fmt.Fprintf(os.Stderr,
							"x protocol read error: %s\n", err)
						onError()
						return
					}
					c.cookies[seq].replyChan <- bigbuf
				} else {
					c.cookies[seq].replyChan <- buf
				}
			default:
				c.events.queue(buf)
				select {
				case c.eventChan <- true:
				default:
				}
			}
		}
	}()
}

func (c *Conn) waitForReply(cookie *Cookie) ([]byte, error) {
	if cookie == nil {
		panic("nil cookie")
	}
	if _, ok := c.cookies[cookie.id]; !ok {
		panic("waiting for a cookie that will never come")
	}
	select {
	case reply := <-cookie.replyChan:
		return reply, nil
	case err := <-cookie.errorChan:
		return nil, err
	}
	panic("unreachable")
}

// WaitForEvent returns the next event from the server.
// It will block until an event is available.
func (c *Conn) WaitForEvent() (Event, error) {
	for {
		if reply := c.events.dequeue(c); reply != nil {
			evCode := reply[0] & 0x7f
			return newEventFuncs[int(evCode)](reply), nil
		}
		if !<-c.eventChan {
			return nil, errors.New("Event channel has been closed.")
		}
	}
	panic("unreachable")
}

// PollForEvent returns the next event from the server if one is available in the internal queue.
// It will not read from the connection, so you must call WaitForEvent to receive new events.
// Only use this function to empty the queue without blocking.
func (c *Conn) PollForEvent() (Event, error) {
	if reply := c.events.dequeue(c); reply != nil {
		evCode := reply[0] & 0x7f
		return newEventFuncs[int(evCode)](reply), nil
	}
	return nil, nil
}

