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
	"strconv"
	"strings"
	"sync"
)

const (
	readBuffer  = 100
	writeBuffer = 100
)

// A Conn represents a connection to an X server.
// Only one goroutine should use a Conn's methods at a time.
type Conn struct {
	host          string
	conn          net.Conn
	nextId        Id
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

	newIdLock   sync.Mutex
	writeLock   sync.Mutex
	dequeueLock sync.Mutex
	cookieLock  sync.Mutex
	extLock     sync.Mutex
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

// Event is an interface that can contain any of the events returned by the server.
// Use a type assertion switch to extract the Event structs.
type Event interface{}

// Error contains protocol errors returned to us by the X server.
type Error struct {
	Detail uint8
	Major  uint8
	Minor  uint16
	Cookie uint16
	Id     Id
}

func (e *Error) Error() string {
	return fmt.Sprintf("Bad%s (major=%d minor=%d cookie=%d id=0x%x)",
		errorNames[e.Detail], e.Major, e.Minor, e.Cookie, e.Id)
}

// NewID generates a new unused ID for use with requests like CreateWindow.
func (c *Conn) NewId() Id {
	c.newIdLock.Lock()
	defer c.newIdLock.Unlock()

	id := c.nextId
	// TODO: handle ID overflow
	c.nextId++
	return id
}

// RegisterExtension adds the respective extension's major op code to
// the extensions map.
func (c *Conn) RegisterExtension(name string) error {
	nameUpper := strings.ToUpper(name)
	reply, err := c.QueryExtension(nameUpper)

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
				err := &Error{
					Detail: buf[1],
					Cookie: uint16(get16(buf[2:])),
					Id:     Id(get32(buf[4:])),
					Minor:  get16(buf[8:]),
					Major:  buf[10],
				}
				if cookie, ok := c.cookies[err.Cookie]; ok {
					cookie.errorChan <- err
				} else {
					fmt.Fprintf(os.Stderr, "x protocol error: %s\n", err)
				}
			case 1:
				seq := uint16(get16(buf[2:]))
				if _, ok := c.cookies[seq]; !ok {
					continue
				}

				size := get32(buf[4:])
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
			return parseEvent(reply)
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
		return parseEvent(reply)
	}
	return nil, nil
}

// Dial connects to the X server given in the 'display' string.
// If 'display' is empty it will be taken from os.Getenv("DISPLAY").
//
// Examples:
//	Dial(":1")                 // connect to net.Dial("unix", "", "/tmp/.X11-unix/X1")
//	Dial("/tmp/launch-123/:0") // connect to net.Dial("unix", "", "/tmp/launch-123/:0")
//	Dial("hostname:2.1")       // connect to net.Dial("tcp", "", "hostname:6002")
//	Dial("tcp/hostname:1.0")   // connect to net.Dial("tcp", "", "hostname:6001")
func Dial(display string) (*Conn, error) {
	c, err := connect(display)
	if err != nil {
		return nil, err
	}

	// Get authentication data
	authName, authData, err := readAuthority(c.host, c.display)
	noauth := false
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not get authority info: %v\n", err)
		fmt.Fprintf(os.Stderr, "Trying connection without authority info...\n")
		authName = ""
		authData = []byte{}
		noauth = true
	}

	// Assume that the authentication protocol is "MIT-MAGIC-COOKIE-1".
	if !noauth && (authName != "MIT-MAGIC-COOKIE-1" || len(authData) != 16) {
		return nil, errors.New("unsupported auth protocol " + authName)
	}

	buf := make([]byte, 12+pad(len(authName))+pad(len(authData)))
	buf[0] = 0x6c
	buf[1] = 0
	put16(buf[2:], 11)
	put16(buf[4:], 0)
	put16(buf[6:], uint16(len(authName)))
	put16(buf[8:], uint16(len(authData)))
	put16(buf[10:], 0)
	copy(buf[12:], []byte(authName))
	copy(buf[12+pad(len(authName)):], authData)
	if _, err = c.conn.Write(buf); err != nil {
		return nil, err
	}

	head := make([]byte, 8)
	if _, err = io.ReadFull(c.conn, head[0:8]); err != nil {
		return nil, err
	}
	code := head[0]
	reasonLen := head[1]
	major := get16(head[2:])
	minor := get16(head[4:])
	dataLen := get16(head[6:])

	if major != 11 || minor != 0 {
		return nil, errors.New(fmt.Sprintf("x protocol version mismatch: %d.%d", major, minor))
	}

	buf = make([]byte, int(dataLen)*4+8, int(dataLen)*4+8)
	copy(buf, head)
	if _, err = io.ReadFull(c.conn, buf[8:]); err != nil {
		return nil, err
	}

	if code == 0 {
		reason := buf[8 : 8+reasonLen]
		return nil, errors.New(fmt.Sprintf("x protocol authentication refused: %s", string(reason)))
	}

	getSetupInfo(buf, &c.Setup)

	if c.defaultScreen >= len(c.Setup.Roots) {
		c.defaultScreen = 0
	}

	c.nextId = Id(c.Setup.ResourceIdBase)
	c.nextCookie = 1
	c.cookies = make(map[uint16]*Cookie)
	c.events = queue{make([][]byte, 100), 0, 0}
	c.extensions = make(map[string]byte)

	c.newReadChannels()
	c.newRequestChannels()
	return c, nil
}

// Close closes the connection to the X server.
func (c *Conn) Close() { c.conn.Close() }

func connect(display string) (*Conn, error) {
	if len(display) == 0 {
		display = os.Getenv("DISPLAY")
	}

	display0 := display
	if len(display) == 0 {
		return nil, errors.New("empty display string")
	}

	colonIdx := strings.LastIndex(display, ":")
	if colonIdx < 0 {
		return nil, errors.New("bad display string: " + display0)
	}

	var protocol, socket string
	c := new(Conn)

	if display[0] == '/' {
		socket = display[0:colonIdx]
	} else {
		slashIdx := strings.LastIndex(display, "/")
		if slashIdx >= 0 {
			protocol = display[0:slashIdx]
			c.host = display[slashIdx+1 : colonIdx]
		} else {
			c.host = display[0:colonIdx]
		}
	}

	display = display[colonIdx+1 : len(display)]
	if len(display) == 0 {
		return nil, errors.New("bad display string: " + display0)
	}

	var scr string
	dotIdx := strings.LastIndex(display, ".")
	if dotIdx < 0 {
		c.display = display[0:]
	} else {
		c.display = display[0:dotIdx]
		scr = display[dotIdx+1:]
	}

	dispnum, err := strconv.Atoi(c.display)
	if err != nil || dispnum < 0 {
		return nil, errors.New("bad display string: " + display0)
	}

	if len(scr) != 0 {
		c.defaultScreen, err = strconv.Atoi(scr)
		if err != nil {
			return nil, errors.New("bad display string: " + display0)
		}
	}

	// Connect to server
	if len(socket) != 0 {
		c.conn, err = net.Dial("unix", socket+":"+c.display)
	} else if len(c.host) != 0 {
		if protocol == "" {
			protocol = "tcp"
		}
		c.conn, err = net.Dial(protocol, c.host+":"+strconv.Itoa(6000+dispnum))
	} else {
		c.conn, err = net.Dial("unix", "/tmp/.X11-unix/X"+c.display)
	}

	if err != nil {
		return nil, errors.New("cannot connect to " + display0 + ": " + err.Error())
	}
	return c, nil
}
