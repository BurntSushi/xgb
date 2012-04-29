package main

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"log"
	"strings"
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

// TypePrefix searches the parsed XML for a type matching 'needle'.
// It then returns the appropriate prefix to be used in source code.
// Note that the core X protocol *is* a namespace, but does not have a prefix.
// Also note that you should probably check the BaseTypeMap and TypeMap
// before calling this function.
func (c *Context) TypePrefix(needle Type) string {
	// If this is xproto, quit. No prefixes needed.
	if c.xml.Header == "xproto" {
		return ""
	}

	// First check for the type in the current namespace.
	if c.xml.HasType(needle) {
		return strings.Title(c.xml.Header)
	}

	// Now check each of the imports...
	for _, imp := range c.xml.Imports {
		if imp.xml.Header != "xproto" && imp.xml.HasType(needle) {
			return strings.Title(imp.xml.Header)
		}
	}

	return ""
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

	// for _, req := range c.xml.Requests { 
		// if req.Name != "CreateContext" && req.Name != "MakeCurrent" { 
			// continue 
		// } 
		// log.Println(req.Name) 
		// for _, field := range req.Fields { 
			// log.Println("\t", field.XMLName.Local, field.Type.Morph(c)) 
		// } 
	// } 
}
