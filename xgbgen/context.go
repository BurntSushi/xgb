package main

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"log"
)

type Context struct {
	protocol *Protocol
	out      *bytes.Buffer
}

func newContext() *Context {
	return &Context{
		out: bytes.NewBuffer([]byte{}),
	}
}

// Putln calls put and adds a new line to the end of 'format'.
func (c *Context) Putln(format string, v ...interface{}) {
	c.Put(format+"\n", v...)
}

// Put is a short alias to write to 'out'.
func (c *Context) Put(format string, v ...interface{}) {
	_, err := c.out.WriteString(fmt.Sprintf(format, v...))
	if err != nil {
		log.Fatalf("There was an error writing to context buffer: %s", err)
	}
}

// Morph is the big daddy of them all. It takes in an XML byte slice,
// parse it, transforms the XML types into more usable types,
// and writes Go code to the 'out' buffer.
func (c *Context) Morph(xmlBytes []byte) {
	parsedXml := &XML{}
	err := xml.Unmarshal(xmlBytes, parsedXml)
	if err != nil {
		log.Fatal(err)
	}

	// Parse all imports
	parsedXml.Imports.Eval()

	// Translate XML types to nice types
	c.protocol = parsedXml.Translate()

	// Now write Go source code
	for _, typ := range c.protocol.Types {
		typ.Define(c)
	}
	for _, req := range c.protocol.Requests {
		req.Define(c)
	}
}
