package main

// Error types
func (e *Error) Define(c *Context) {
	c.Putln("// Error definition %s (%d)", e.SrcName(), e.Number)
	c.Putln("")
}

func (e *Error) Read(c *Context, prefix string) {
	c.Putln("// Error read %s", e.SrcName())
}

func (e *Error) Write(c *Context, prefix string) {
	c.Putln("// Error write %s", e.SrcName())
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

	// Write defines a function that transoforms this error struct into
	// a byte slice.
	e.Write(c)

	// Makes sure that this error type is an Error interface.
	c.Putln("func (err %s) ImplementsError() { }", e.ErrType())
	c.Putln("")

	// Let's the XGB know how to read this error.
	c.Putln("func init() {")
	c.Putln("newErrorFuncs[%d] = New%s", e.Number, e.SrcName())
	c.Putln("}")
	c.Putln("")
}

func (e *ErrorCopy) Read(c *Context) {
	c.Putln("func New%s(buf []byte) %s {", e.SrcName(), e.ErrType())
	c.Putln("return (%s)(New%s(buf))", e.ErrType(), e.Old.SrcName())
	c.Putln("}")
	c.Putln("")
}

func (e *ErrorCopy) Write(c *Context) {
	c.Putln("func (err %s) Bytes() []byte {", e.ErrType())
	c.Putln("return (%s)(err).Bytes()", e.Old.(*Error).ErrType())
	c.Putln("}")
	c.Putln("")
}
