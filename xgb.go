package xgb

import (
	"errors"
	"io"
	"log"
	"net"
	"os"
	"sync"
)

var logger = log.New(os.Stderr, "XGB: ", 0)

const (
	// cookieBuffer represents the queue size of cookies existing at any
	// point in time. The size of the buffer is really only important when
	// there are many requests without replies made in sequence. Once the
	// buffer fills, a round trip request is made to clear the buffer.
	cookieBuffer = 1000

	// xidBuffer represents the queue size of the xid channel.
	// I don't think this value matters much, since xid generation is not
	// that expensive.
	xidBuffer = 5

	// seqBuffer represents the queue size of the sequence number channel.
	// I don't think this value matters much, since sequence number generation 
	// is not that expensive.
	seqBuffer = 5

	// reqBuffer represents the queue size of the number of requests that
	// can be made until new ones block. This value seems OK.
	reqBuffer = 100

	// eventBuffer represents the queue size of the number of events or errors
	// that can be loaded off the wire and not grabbed with WaitForEvent
	// until reading an event blocks. This value should be big enough to handle
	// bursts of events.
	eventBuffer = 500
)

// A Conn represents a connection to an X server.
type Conn struct {
	host          string
	conn          net.Conn
	display       string
	defaultScreen int
	Setup         SetupInfo

	eventChan  chan eventOrError
	cookieChan chan *cookie
	xidChan    chan xid
	seqChan    chan uint16
	reqChan    chan *request

	extLock    sync.Mutex
	extensions map[string]byte
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
// NewConn(":1") -> net.Dial("unix", "", "/tmp/.X11-unix/X1")
// NewConn("/tmp/launch-123/:0") -> net.Dial("unix", "", "/tmp/launch-123/:0")
// NewConn("hostname:2.1") -> net.Dial("tcp", "", "hostname:6002")
// NewConn("tcp/hostname:1.0") -> net.Dial("tcp", "", "hostname:6001")
func NewConnDisplay(display string) (*Conn, error) {
	conn := &Conn{}

	// First connect. This reads authority, checks DISPLAY environment
	// variable, and loads the initial Setup info.
	err := conn.connect(display)
	if err != nil {
		return nil, err
	}

	conn.extensions = make(map[string]byte)

	conn.cookieChan = make(chan *cookie, cookieBuffer)
	conn.xidChan = make(chan xid, xidBuffer)
	conn.seqChan = make(chan uint16, seqBuffer)
	conn.reqChan = make(chan *request, reqBuffer)
	conn.eventChan = make(chan eventOrError, eventBuffer)

	go conn.generateXIds()
	go conn.generateSeqIds()
	go conn.sendRequests()
	go conn.readResponses()

	return conn, nil
}

// Close closes the connection to the X server.
func (c *Conn) Close() {
	c.conn.Close()
}

// DefaultScreen returns the Screen info for the default screen, which is
// 0 or the one given in the display argument to Dial.
func (c *Conn) DefaultScreen() *ScreenInfo {
	return &c.Setup.Roots[c.defaultScreen]
}

// Id is used for all X identifiers, such as windows, pixmaps, and GCs.
type Id uint32

// Event is an interface that can contain any of the events returned by the
// server. Use a type assertion switch to extract the Event structs.
type Event interface {
	ImplementsEvent()
	Bytes() []byte
	String() string
}

type newEventFun func(buf []byte) Event

// newEventFuncs is a map from event numbers to functions that create
// the corresponding event.
var newEventFuncs = make(map[int]newEventFun)

// newExtEventFuncs is a temporary map that stores event constructor functions
// for each extension. When an extension is initialized, each event for that
// extension is added to the 'newEventFuncs' map.
var newExtEventFuncs = make(map[string]map[int]newEventFun)

// Error is an interface that can contain any of the errors returned by
// the server. Use a type assertion switch to extract the Error structs.
type Error interface {
	ImplementsError()
	SequenceId() uint16
	BadId() Id
	Error() string
}

type newErrorFun func(buf []byte) Error

// newErrorFuncs is a map from error numbers to functions that create
// the corresponding error.
var newErrorFuncs = make(map[int]newErrorFun)

// newExtErrorFuncs is a temporary map that stores error constructor functions
// for each extension. When an extension is initialized, each error for that
// extension is added to the 'newErrorFuncs' map.
var newExtErrorFuncs = make(map[string]map[int]newErrorFun)

// eventOrError corresponds to values that can be either an event or an
// error.
type eventOrError interface{}

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
// channel. If no new resource id can be generated, id is set to 0 and a
// non-nil error is set in xid.err.
type xid struct {
	id  Id
	err error
}

// generateXids sends new Ids down the channel for NewId to use.
// This needs to be updated to use the XC Misc extension once we run out of
// new ids.
// Thanks to libxcb/src/xcb_xid.c. This code is greatly inspired by it.
func (conn *Conn) generateXIds() {
	// This requires some explanation. From the horse's mouth:
	// "The resource-id-mask contains a single contiguous set of bits (at least 
	// 18).  The client allocates resource IDs for types WINDOW, PIXMAP, 
	// CURSOR, FONT, GCONTEXT, and COLORMAP by choosing a value with only some 
	// subset of these bits set and ORing it with resource-id-base. Only values 
	// constructed in this way can be used to name newly created resources over 
	// this connection."
	// So for example (using 8 bit integers), the mask might look like:
	// 00111000
	// So that valid values would be 00101000, 00110000, 00001000, and so on.
	// Thus, the idea is to increment it by the place of the last least
	// significant '1'. In this case, that value would be 00001000. To get
	// that value, we can AND the original mask with its two's complement:
	// 00111000 & 11001000 = 00001000.
	// And we use that value to increment the last resource id to get a new one.
	// (And then, of course, we OR it with resource-id-base.)
	inc := conn.Setup.ResourceIdMask & -conn.Setup.ResourceIdMask
	max := conn.Setup.ResourceIdMask
	last := uint32(0)
	for {
		// TODO: Use the XC Misc extension to look for released ids.
		if last > 0 && last >= max-inc+1 {
			conn.xidChan <- xid{
				id: Id(0),
				err: errors.New("There are no more available resource" +
					"identifiers."),
			}
		}

		last += inc
		conn.xidChan <- xid{
			id:  Id(last | conn.Setup.ResourceIdBase),
			err: nil,
		}
	}
}

// newSeqId fetches the next sequence id from the Conn.seqChan channel.
func (c *Conn) newSequenceId() uint16 {
	return <-c.seqChan
}

// generateSeqIds returns new sequence ids.
// A sequence id is generated for *every* request. It's the identifier used
// to match up replies with requests.
// Since sequence ids can only be 16 bit integers we start over at zero when it 
// comes time to wrap.
// N.B. As long as the cookie buffer is less than 2^16, there are no limitations
// on the number (or kind) of requests made in sequence.
func (c *Conn) generateSeqIds() {
	seqid := uint16(1)
	for {
		c.seqChan <- seqid
		if seqid == uint16((1<<16)-1) {
			seqid = 0
		} else {
			seqid++
		}
	}
}

// request encapsulates a buffer of raw bytes (containing the request data)
// and a cookie, which when combined represents a single request.
// The cookie is used to match up the reply/error.
type request struct {
	buf    []byte
	cookie *cookie
}

// newRequest takes the bytes an a cookie, constructs a request type,
// and sends it over the Conn.reqChan channel.
// Note that the sequence number is added to the cookie after it is sent
// over the request channel.
func (c *Conn) newRequest(buf []byte, cookie *cookie) {
	c.reqChan <- &request{buf: buf, cookie: cookie}
}

// sendRequests is run as a single goroutine that takes requests and writes
// the bytes to the wire and adds the cookie to the cookie queue.
func (c *Conn) sendRequests() {
	for req := range c.reqChan {
		// ho there! if the cookie channel is nearly full, force a round
		// trip to clear out the cookie buffer.
		// Note that we circumvent the request channel, because we're *in*
		// the request channel.
		if len(c.cookieChan) == cookieBuffer-1 {
			cookie := c.newCookie(true, true)
			cookie.Sequence = c.newSequenceId()
			c.cookieChan <- cookie
			c.writeBuffer(c.getInputFocusRequest())
			GetInputFocusCookie{cookie}.Reply() // wait for the buffer to clear
		}

		req.cookie.Sequence = c.newSequenceId()
		c.cookieChan <- req.cookie
		c.writeBuffer(req.buf)
	}
}

// writeBuffer is a convenience function for writing a byte slice to the wire.
func (c *Conn) writeBuffer(buf []byte) {
	if _, err := c.conn.Write(buf); err != nil {
		logger.Printf("Write error: %s", err)
		logger.Fatal("A write error is unrecoverable. Exiting...")
	}
}

// readResponses is a goroutine that reads events, errors and
// replies off the wire.
// When an event is read, it is always added to the event channel.
// When an error is read, if it corresponds to an existing checked cookie,
// it is sent to that cookie's error channel. Otherwise it is added to the
// event channel.
// When a reply is read, it is added to the corresponding cookie's reply
// channel. (It is an error if no such cookie exists in this case.)
// Finally, cookies that came "before" this reply are always cleaned up.
func (c *Conn) readResponses() {
	var (
		err        Error
		event      Event
		seq        uint16
		replyBytes []byte
	)

	buf := make([]byte, 32)
	for {
		err, event, seq = nil, nil, 0

		if _, err := io.ReadFull(c.conn, buf); err != nil {
			logger.Printf("Read error: %s", err)
			logger.Fatal("A read error is unrecoverable. Exiting...")
		}

		switch buf[0] {
		case 0: // This is an error
			// Use the constructor function for this error (that is auto
			// generated) by looking it up by the error number.
			newErrFun, ok := newErrorFuncs[int(buf[1])]
			if !ok {
				logger.Printf("BUG: Could not find error constructor function "+
					"for error with number %d.", buf[1])
				continue
			}
			err = newErrFun(buf)
			seq = err.SequenceId()

			// This error is either sent to the event channel or a specific
			// cookie's error channel below.
		case 1: // This is a reply
			seq = Get16(buf[2:])

			// check to see if this reply has more bytes to be read
			size := Get32(buf[4:])
			if size > 0 {
				byteCount := 32 + size*4
				biggerBuf := make([]byte, byteCount)
				copy(biggerBuf[:32], buf)
				if _, err := io.ReadFull(c.conn, biggerBuf[32:]); err != nil {
					logger.Printf("Read error: %s", err)
					logger.Fatal("A read error is unrecoverable. Exiting...")
				}
				replyBytes = biggerBuf
			} else {
				replyBytes = buf
			}

			// This reply is sent to its corresponding cookie below.
		default: // This is an event
			// Use the constructor function for this event (like for errors,
			// and is also auto generated) by looking it up by the event number.
			// Note that we AND the event number with 127 so that we ignore
			// the most significant bit (which is set when it was sent from
			// a SendEvent request).
			evNum := int(buf[0] & 127)
			newEventFun, ok := newEventFuncs[evNum]
			if !ok {
				logger.Printf("BUG: Could not find event construct function "+
					"for event with number %d.", evNum)
				continue
			}

			event = newEventFun(buf)

			// Put the event into the queue.
			// FIXME: I'm not sure if using a goroutine here to guarantee
			// a non-blocking send is the right way to go. I should implement
			// a proper dynamic queue.
			if cap(c.eventChan) == len(c.eventChan) {
				go func() {
					c.eventChan <- event
				}()
			} else {
				c.eventChan <- event
			}

			// No more processing for events.
			continue
		}

		// At this point, we have a sequence number and we're either
		// processing an error or a reply, which are both responses to
		// requests. So all we have to do is find the cookie corresponding
		// to this error/reply, and send the appropriate data to it.
		// In doing so, we make sure that any cookies that came before it
		// are marked as successful if they are void and checked.
		// If there's a cookie that requires a reply that is before this
		// reply, then something is wrong.
		for cookie := range c.cookieChan {
			// This is the cookie we're looking for. Process and break.
			if cookie.Sequence == seq {
				if err != nil { // this is an error to a request
					// synchronous processing
					if cookie.errorChan != nil {
						cookie.errorChan <- err
					} else { // asynchronous processing
						c.eventChan <- err
						// if this is an unchecked reply, ping the cookie too
						if cookie.pingChan != nil {
							cookie.pingChan <- true
						}
					}
				} else { // this is a reply
					if cookie.replyChan == nil {
						logger.Printf("Reply with sequence id %d does not "+
							"have a cookie with a valid reply channel.", seq)
						continue
					} else {
						cookie.replyChan <- replyBytes
					}
				}
				break
			}

			switch {
			// Checked requests with replies
			case cookie.replyChan != nil && cookie.errorChan != nil:
				logger.Printf("Found cookie with sequence id %d that is "+
					"expecting a reply but will never get it. Currently "+
					"on sequence number %d", cookie.Sequence, seq)
			// Unchecked requests with replies
			case cookie.replyChan != nil && cookie.pingChan != nil:
				logger.Printf("Found cookie with sequence id %d that is "+
					"expecting a reply (and not an error) but will never "+
					"get it. Currently on sequence number %d",
					cookie.Sequence, seq)
			// Checked requests without replies
			case cookie.pingChan != nil && cookie.errorChan != nil:
				cookie.pingChan <- true
				// Unchecked requests without replies don't have any channels,
				// so we can't do anything with them except let them pass by.
			}
		}
	}
}

// processEventOrError takes an eventOrError, type switches on it,
// and returns it in Go idiomatic style.
func processEventOrError(everr eventOrError) (Event, Error) {
	switch ee := everr.(type) {
	case Event:
		return ee, nil
	case Error:
		return nil, ee
	default:
		logger.Printf("Invalid event/error type: %T", everr)
		return nil, nil
	}
	panic("unreachable")
}

// WaitForEvent returns the next event from the server.
// It will block until an event is available.
func (c *Conn) WaitForEvent() (Event, Error) {
	return processEventOrError(<-c.eventChan)
}

// PollForEvent returns the next event from the server if one is available in 
// the internal queue.
// It will not block.
func (c *Conn) PollForEvent() (Event, Error) {
	select {
	case everr := <-c.eventChan:
		return processEventOrError(everr)
	default:
		return nil, nil
	}
	panic("unreachable")
}
