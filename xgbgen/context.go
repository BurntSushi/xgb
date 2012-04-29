package main

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"log"
)

type Context struct {
	xml *XML
	out *bytes.Buffer
}

func newContext() *Context {
	return &Context{
		xml: &XML{},
		out: bytes.NewBuffer([]byte{}),
	}
}

// Putln calls put and adds a new line to the end of 'format'.
func (c *Context) Putln(format string, v ...interface{}) {
	c.Put(format + "\n", v...)
}

// Put is a short alias to write to 'out'.
func (c *Context) Put(format string, v ...interface{}) {
	_, err := c.out.WriteString(fmt.Sprintf(format, v...))
	if err != nil {
		log.Fatalf("There was an error writing to context buffer: %s", err)
	}
}


// Translate is the big daddy of them all. It takes in an XML byte slice
// and writes Go code to the 'out' buffer.
func (c *Context) Translate(xmlBytes []byte) {
	err := xml.Unmarshal(xmlBytes, c.xml)
	if err != nil {
		log.Fatal(err)
	}

	// Parse all imports
	c.xml.Imports.Eval()

	// Make sure all top level enumerations have expressions
	// (For when there are empty items.)
	c.xml.Enums.Eval()

	// It's Morphin' Time!
	c.xml.Morph(c)
}
