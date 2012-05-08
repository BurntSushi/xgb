package main

import (
	"fmt"
)

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

	// Makes sure this error type implements the xgb.Error interface.
	e.ImplementsError(c)

	// Let's the XGB event loop read this error.
	c.Putln("func init() {")
	if c.protocol.isExt() {
		c.Putln("newExtErrorFuncs[\"%s\"][%d] = New%s",
			c.protocol.ExtXName, e.Number, e.ErrType())
	} else {
		c.Putln("newErrorFuncs[%d] = New%s", e.Number, e.ErrType())
	}
	c.Putln("}")
	c.Putln("")
}

func (e *Error) Read(c *Context) {
	c.Putln("// Error read %s", e.SrcName())
	c.Putln("func New%s(buf []byte) Error {", e.ErrType())
	c.Putln("v := %s{}", e.ErrType())
	c.Putln("v.NiceName = \"%s\"", e.SrcName())
	c.Putln("")
	c.Putln("b := 1 // skip error determinant")
	c.Putln("b += 1 // don't read error number")
	c.Putln("")
	c.Putln("v.Sequence = Get16(buf[b:])")
	c.Putln("b += 2")
	c.Putln("")
	for _, field := range e.Fields {
		field.Read(c, "v.")
		c.Putln("")
	}
	c.Putln("return v")
	c.Putln("}")
	c.Putln("")
}

// ImplementsError writes functions to implement the XGB Error interface.
func (e *Error) ImplementsError(c *Context) {
	c.Putln("func (err %s) ImplementsError() { }", e.ErrType())
	c.Putln("")
	c.Putln("func (err %s) SequenceId() uint16 {", e.ErrType())
	c.Putln("return err.Sequence")
	c.Putln("}")
	c.Putln("")
	c.Putln("func (err %s) BadId() Id {", e.ErrType())
	if !c.protocol.isExt() {
		c.Putln("return Id(err.BadValue)")
	} else {
		c.Putln("return 0")
	}
	c.Putln("}")
	c.Putln("")
	c.Putln("func (err %s) Error() string {", e.ErrType())
	ErrorFieldString(c, e.Fields, e.ErrConst())
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

	// Makes sure this error type implements the xgb.Error interface.
	e.ImplementsError(c)

	// Let's the XGB know how to read this error.
	c.Putln("func init() {")
	if c.protocol.isExt() {
		c.Putln("newExtErrorFuncs[\"%s\"][%d] = New%s",
			c.protocol.ExtXName, e.Number, e.ErrType())
	} else {
		c.Putln("newErrorFuncs[%d] = New%s", e.Number, e.ErrType())
	}
	c.Putln("}")
	c.Putln("")
}

func (e *ErrorCopy) Read(c *Context) {
	c.Putln("func New%s(buf []byte) Error {", e.ErrType())
	c.Putln("v := %s(New%s(buf).(%s))",
		e.ErrType(), e.Old.(*Error).ErrType(), e.Old.(*Error).ErrType())
	c.Putln("v.NiceName = \"%s\"", e.SrcName())
	c.Putln("return v")
	c.Putln("}")
	c.Putln("")
}

// ImplementsError writes functions to implement the XGB Error interface.
func (e *ErrorCopy) ImplementsError(c *Context) {
	c.Putln("func (err %s) ImplementsError() { }", e.ErrType())
	c.Putln("")
	c.Putln("func (err %s) SequenceId() uint16 {", e.ErrType())
	c.Putln("return err.Sequence")
	c.Putln("}")
	c.Putln("")
	c.Putln("func (err %s) BadId() Id {", e.ErrType())
	c.Putln("return Id(err.BadValue)")
	c.Putln("}")
	c.Putln("")
	c.Putln("func (err %s) Error() string {", e.ErrType())
	ErrorFieldString(c, e.Old.(*Error).Fields, e.ErrConst())
	c.Putln("}")
	c.Putln("")
}

// ErrorFieldString works for both Error and ErrorCopy. It assembles all of the
// fields in an error and formats them into a single string.
func ErrorFieldString(c *Context, fields []Field, errName string) {
	c.Putln("fieldVals := make([]string, 0, %d)", len(fields))
	c.Putln("fieldVals = append(fieldVals, \"NiceName: \" + err.NiceName)")
	c.Putln("fieldVals = append(fieldVals, "+
		"sprintf(\"Sequence: %s\", err.Sequence))", "%d")
	for _, field := range fields {
		switch field.(type) {
		case *PadField:
			continue
		default:
			if field.SrcType() == "string" {
				c.Putln("fieldVals = append(fieldVals, \"%s: \" + err.%s)",
					field.SrcName(), field.SrcName())
			} else {
				format := fmt.Sprintf("sprintf(\"%s: %s\", err.%s)",
					field.SrcName(), "%d", field.SrcName())
				c.Putln("fieldVals = append(fieldVals, %s)", format)
			}
		}
	}
	c.Putln("return \"%s {\" + stringsJoin(fieldVals, \", \") + \"}\"", errName)
}
