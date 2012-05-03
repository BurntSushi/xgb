package main

// Event types
func (e *Event) Define(c *Context) {
	c.Putln("// Event definition %s (%d)", e.SrcName(), e.Number)
	c.Putln("// Size: %s", e.Size())
	c.Putln("")
	c.Putln("const %s = %d", e.SrcName(), e.Number)
	c.Putln("")
	c.Putln("type %s struct {", e.EvType())
	if !e.NoSequence {
		c.Putln("Sequence uint16")
	}
	for _, field := range e.Fields {
		field.Define(c)
	}
	c.Putln("}")
	c.Putln("")

	// Read defines a function that transforms a byte slice into this
	// event struct.
	e.Read(c)

	// Write defines a function that transforms this event struct into
	// a byte slice.
	e.Write(c)

	// Makes sure that this event type is an Event interface.
	c.Putln("func (v %s) ImplementsEvent() { }", e.EvType())
	c.Putln("")

	// Let's the XGB event loop read this event.
	c.Putln("func init() {")
	c.Putln("newEventFuncs[%d] = New%s", e.Number, e.EvType())
	c.Putln("}")
	c.Putln("")
}

func (e *Event) Read(c *Context) {
	c.Putln("// Event read %s", e.SrcName())
	c.Putln("func New%s(buf []byte) Event {", e.EvType())
	c.Putln("v := %s{}", e.EvType())
	c.Putln("b := 1 // don't read event number")
	c.Putln("")
	for i, field := range e.Fields {
		if i == 1 && !e.NoSequence {
			c.Putln("v.Sequence = Get16(buf[b:])")
			c.Putln("b += 2")
			c.Putln("")
		}
		field.Read(c, "v.")
		c.Putln("")
	}
	c.Putln("return v")
	c.Putln("}")
	c.Putln("")
}

func (e *Event) Write(c *Context) {
	c.Putln("// Event write %s", e.SrcName())
	c.Putln("func (v %s) Bytes() []byte {", e.EvType())
	c.Putln("buf := make([]byte, %s)", e.Size())
	c.Putln("b := 0")
	c.Putln("")
	c.Putln("// write event number")
	c.Putln("buf[b] = %d", e.Number)
	c.Putln("b += 1")
	c.Putln("")
	for i, field := range e.Fields {
		if i == 1 && !e.NoSequence {
			c.Putln("b += 2 // skip sequence number")
			c.Putln("")
		}
		field.Write(c, "v.")
		c.Putln("")
	}
	c.Putln("return buf")
	c.Putln("}")
	c.Putln("")
}

// EventCopy types
func (e *EventCopy) Define(c *Context) {
	c.Putln("// EventCopy definition %s (%d)", e.SrcName(), e.Number)
	c.Putln("")
	c.Putln("const %s = %d", e.SrcName(), e.Number)
	c.Putln("")
	c.Putln("type %s %s", e.EvType(), e.Old.(*Event).EvType())
	c.Putln("")

	// Read defines a function that transforms a byte slice into this
	// event struct.
	e.Read(c)

	// Write defines a function that transoforms this event struct into
	// a byte slice.
	e.Write(c)

	// Makes sure that this event type is an Event interface.
	c.Putln("func (v %s) ImplementsEvent() { }", e.EvType())
	c.Putln("")

	// Let's the XGB event loop read this event.
	c.Putln("func init() {")
	c.Putln("newEventFuncs[%d] = New%s", e.Number, e.EvType())
	c.Putln("}")
	c.Putln("")
}

func (e *EventCopy) Read(c *Context) {
	c.Putln("func New%s(buf []byte) Event {", e.EvType())
	c.Putln("return %s(New%s(buf).(%s))",
		e.EvType(), e.Old.(*Event).EvType(), e.Old.(*Event).EvType())
	c.Putln("}")
	c.Putln("")
}

func (e *EventCopy) Write(c *Context) {
	c.Putln("func (v %s) Bytes() []byte {", e.EvType())
	c.Putln("return %s(v).Bytes()", e.Old.(*Event).EvType())
	c.Putln("}")
	c.Putln("")
}
