package xgb

import (
	"errors"
)

type cookie struct {
	Sequence        uint16
	replyChan chan []byte
	errorChan chan error
	pingChan chan bool
}

func (c *Conn) newCookie(checked, reply bool) cookie {
	cookie := cookie{
		Sequence: c.newSequenceId(),
		replyChan: nil,
		errorChan: nil,
		pingChan: nil,
	}

	// There are four different kinds of cookies:
	// Checked requests with replies get a reply channel and an error channel.
	// Unchecked requests with replies get a reply channel and a ping channel.
	// Checked requests w/o replies get a ping channel and an error channel.
	// Unchecked requests w/o replies get no channels.
	// The reply channel is used to send reply data.
	// The error channel is used to send error data.
	// The ping channel is used when one of the 'reply' or 'error' channels
	// is missing but the other is present. The ping channel is way to force
	// the blocking to stop and basically say "the error has been received
	// in the main event loop" (when the ping channel is coupled with a reply
	// channel) or "the request you made that has no reply was successful"
	// (when the ping channel is coupled with an error channel).
	if checked {
		cookie.errorChan = make(chan error, 1)
		if !reply {
			cookie.pingChan = make(chan bool, 1)
		}
	}
	if reply {
		cookie.replyChan = make(chan []byte, 1)
		if !checked {
			cookie.pingChan = make(chan bool, 1)
		}
	}

	return cookie
}

func (c cookie) reply() ([]byte, error) {
	// checked
	if c.errorChan != nil {
		return c.replyChecked()
	}
	return c.replyUnchecked()
}

func (c cookie) replyChecked() ([]byte, error) {
	if c.replyChan == nil {
		return nil, errors.New("Cannot call 'replyChecked' on a cookie that " +
			"is not expecting a *reply* or an error.")
	}
	if c.errorChan == nil {
		return nil, errors.New("Cannot call 'replyChecked' on a cookie that " +
			"is not expecting a reply or an *error*.")
	}

	select {
	case reply := <-c.replyChan:
		return reply, nil
	case err := <-c.errorChan:
		return nil, err
	}
	panic("unreachable")
}

func (c cookie) replyUnchecked() ([]byte, error) {
	if c.replyChan == nil {
		return nil, errors.New("Cannot call 'replyUnchecked' on a cookie " +
			"that is not expecting a *reply*.")
	}

	select {
	case reply := <-c.replyChan:
		return reply, nil
	case <-c.pingChan:
		return nil, nil
	}
	panic("unreachable")
}

func (c cookie) Check() error {
	if c.replyChan != nil {
		return errors.New("Cannot call 'Check' on a cookie that is " +
			"expecting a *reply*. Use 'Reply' instead.")
	}
	if c.errorChan == nil {
		return errors.New("Cannot call 'Check' on a cookie that is " +
			"not expecting a possible *error*.")
	}

	select {
	case err := <-c.errorChan:
		return err
	case <-c.pingChan:
		return nil
	}
	panic("unreachable")
}

