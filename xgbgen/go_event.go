package main

// Event types
func (e *Event) Define(c *Context) {
	c.Putln("// Event definition %s (%d)", e.SrcName(), e.Number)
}

func (e *Event) Read(c *Context, prefix string) {
	c.Putln("// Event read %s", e.SrcName())
}

func (e *Event) Write(c *Context, prefix string) {
	c.Putln("// Event write %s", e.SrcName())
}

// EventCopy types
func (e *EventCopy) Define(c *Context) {
	c.Putln("// EventCopy definition %s (%d)", e.SrcName(), e.Number)
	c.Putln("")
	c.Putln("const %s = %d", e.SrcName(), e.Number)
	c.Putln("")
	c.Putln("type %s %s", e.EvType(), e.Old.(*Event).EvType())
	c.Putln("")
	c.Putln("func New%s(buf []byte) %s {", e.SrcName(), e.EvType())
	c.Putln("return (%s)(New%s(buf))", e.EvType(), e.Old.SrcName())
	c.Putln("}")
	c.Putln("")
	c.Putln("func (ev %s) ImplementsEvent() { }", e.EvType())
	c.Putln("")
	c.Putln("func (ev %s) Bytes() []byte {", e.EvType())
	c.Putln("return (%s)(ev).Bytes()", e.Old.(*Event).EvType())
	c.Putln("}")
	c.Putln("")
	c.Putln("func init() {")
	c.Putln("newEventFuncs[%d] = New%s", e.Number, e.SrcName())
	c.Putln("}")
	c.Putln("")
}
