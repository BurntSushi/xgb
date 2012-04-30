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
	c.Putln("func New%s(buf []byte) %s {", e.SrcName(), e.ErrType())
	c.Putln("return (%s)(New%s(buf))", e.ErrType(), e.Old.SrcName())
	c.Putln("}")
	c.Putln("")
	c.Putln("func (err %s) ImplementsError() { }", e.ErrType())
	c.Putln("")
	c.Putln("func (err %s) Bytes() []byte {", e.ErrType())
	c.Putln("return (%s)(err).Bytes()", e.Old.(*Error).ErrType())
	c.Putln("}")
	c.Putln("")
	c.Putln("func init() {")
	c.Putln("newErrorFuncs[%d] = New%s", e.Number, e.SrcName())
	c.Putln("}")
	c.Putln("")
}
