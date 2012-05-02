package main

// Error types
func (e *Error) Define(c *Context) {
	c.Putln("// Error definition %s (%d)", e.SrcName(), e.Number)
	c.Putln("// Size: %s", e.Size())
	c.Putln("")
	c.Putln("const %s = %d", e.ErrConst(), e.Number)
	c.Putln("")
	c.Putln("type %s struct {", e.ErrType())
	c.Putln("Sequence uint16")
	c.Putln("NiceName string")
	for _, field := range e.Fields {
		field.Define(c)
	}
	c.Putln("}")
	c.Putln("")

	// Read defines a function that transforms a byte slice into this
	// error struct.
	e.Read(c)

	// Makes sure that this error type is an Error interface.
	c.Putln("func (v %s) ImplementsError() { }", e.ErrType())
	c.Putln("")

	// Let's the XGB event loop read this error.
	c.Putln("func init() {")
	c.Putln("newErrorFuncs[%d] = New%s", e.Number, e.ErrType())
	c.Putln("}")
	c.Putln("")
}

func (e *Error) Read(c *Context) {
	c.Putln("// Error read %s", e.SrcName())
	c.Putln("func New%s(buf []byte) %s {", e.ErrType(), e.ErrType())
	c.Putln("v := %s{}", e.ErrType())
	c.Putln("v.NiceName = \"%s\"", e.SrcName())
	c.Putln("")
	c.Putln("b := 1 // skip error determinant")
	c.Putln("b += 1 // don't read error number")
	c.Putln("")
	c.Putln("v.Sequence = get16(buf[b:])")
	c.Putln("b += 2")
	c.Putln("")
	for _, field := range e.Fields {
		field.Read(c)
		c.Putln("")
	}
	c.Putln("return v")
	c.Putln("}")
	c.Putln("")
}

// ErrorCopy types
func (e *ErrorCopy) Define(c *Context) {
	c.Putln("// ErrorCopy definition %s (%d)", e.SrcName(), e.Number)
	c.Putln("")
	c.Putln("const %s = %d", e.ErrConst(), e.Number)
	c.Putln("")
	c.Putln("type %s %s", e.ErrType(), e.Old.(*Error).ErrType())
	c.Putln("")

	// Read defines a function that transforms a byte slice into this
	// error struct.
	e.Read(c)

	// Makes sure that this error type is an Error interface.
	c.Putln("func (err %s) ImplementsError() { }", e.ErrType())
	c.Putln("")

	// Let's the XGB know how to read this error.
	c.Putln("func init() {")
	c.Putln("newErrorFuncs[%d] = New%s", e.Number, e.ErrType())
	c.Putln("}")
	c.Putln("")
}

func (e *ErrorCopy) Read(c *Context) {
	c.Putln("func New%s(buf []byte) %s {", e.ErrType(), e.ErrType())
	c.Putln("return %s(New%s(buf))", e.ErrType(), e.Old.(*Error).ErrType())
	c.Putln("}")
	c.Putln("")
}
