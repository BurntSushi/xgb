package main

import (
	"fmt"
	"strings"
)

func (r *Request) Define(c *Context) {
	c.Putln("// Request %s", r.SrcName())
	c.Putln("// size: %s", r.Size(c))
	if r.Reply != nil {
		c.Putln("func (c *Conn) %s(%s) (*%s, error) {",
			r.SrcName(), r.ParamNameTypes(), r.ReplyName())
		c.Putln("return c.%s(c.%s(%s))",
			r.ReplyName(), r.ReqName(), r.ParamNames())
		c.Putln("}")
		c.Putln("")

		r.WriteRequest(c)
		r.ReadReply(c)
	} else {
		c.Putln("// Write request to wire for %s", r.SrcName())
		c.Putln("func (c *Conn) %s(%s) {", r.SrcName(), r.ParamNameTypes())
		r.WriteRequestFields(c)
		c.Putln("c.sendRequest(false, buf)")
		c.Putln("}")
		c.Putln("")
	}
}

func (r *Request) ReadReply(c *Context) {
	c.Putln("// Request reply for %s", r.SrcName())
	c.Putln("// size: %s", r.Reply.Size())
	c.Putln("type %s struct {", r.ReplyName())
	c.Putln("Sequence uint16")
	c.Putln("Length uint32")
	for _, field := range r.Reply.Fields {
		field.Define(c)
	}
	c.Putln("}")
	c.Putln("")

	c.Putln("// Read reply %s", r.SrcName())
	c.Putln("func (c *Conn) %s(cook *Cookie) (*%s, error) {",
		r.ReplyName(), r.ReplyName())
	c.Putln("buf, err := c.waitForReply(cook)")
	c.Putln("if err != nil {")
	c.Putln("return nil, err")
	c.Putln("}")
	c.Putln("")
	c.Putln("v := new(%s)", r.ReplyName())
	c.Putln("b := 1 // skip reply determinant")
	c.Putln("")
	for i, field := range r.Reply.Fields {
		if i == 1 {
			c.Putln("v.Sequence = Get16(buf[b:])")
			c.Putln("b += 2")
			c.Putln("")
			c.Putln("v.Length = Get32(buf[b:]) // 4-byte units")
			c.Putln("b += 4")
			c.Putln("")
		}
		field.Read(c, "v.")
		c.Putln("")
	}
	c.Putln("return v, nil")
	c.Putln("}")
	c.Putln("")
}

func (r *Request) WriteRequest(c *Context) {
	c.Putln("// Write request to wire for %s", r.SrcName())
	c.Putln("func (c *Conn) %s(%s) *Cookie {", r.ReqName(), r.ParamNameTypes())
	r.WriteRequestFields(c)
	c.Putln("return c.sendRequest(true, buf)")
	c.Putln("}")
	c.Putln("")
}

func (r *Request) WriteRequestFields(c *Context) {
	c.Putln("size := %s", r.Size(c))
	c.Putln("b := 0")
	c.Putln("buf := make([]byte, size)")
	c.Putln("")
	c.Putln("buf[b] = %d // request opcode", r.Opcode)
	c.Putln("b += 1")
	c.Putln("")
	for i, field := range r.Fields {
		if i == 1 {
			c.Putln("Put16(buf[b:], uint16(size / 4)) "+
				"// write request size in 4-byte units")
			c.Putln("b += 2")
			c.Putln("")
		}
		field.Write(c, "")
		c.Putln("")
	}
}

func (r *Request) ParamNames() string {
	names := make([]string, 0, len(r.Fields))
	for _, field := range r.Fields {
		switch f := field.(type) {
		case *ValueField:
			names = append(names, f.MaskName)
			names = append(names, f.ListName)
		case *PadField:
			continue
		case *ExprField:
			continue
		default:
			names = append(names, fmt.Sprintf("%s", field.SrcName()))
		}
	}
	return strings.Join(names, ",")
}

func (r *Request) ParamNameTypes() string {
	nameTypes := make([]string, 0, len(r.Fields))
	for _, field := range r.Fields {
		switch f := field.(type) {
		case *ValueField:
			// mofos...
			if r.SrcName() != "ConfigureWindow" {
				nameTypes = append(nameTypes,
					fmt.Sprintf("%s %s", f.MaskName, f.MaskType.SrcName()))
			}
			nameTypes = append(nameTypes,
				fmt.Sprintf("%s []uint32", f.ListName))
		case *PadField:
			continue
		case *ExprField:
			continue
		default:
			nameTypes = append(nameTypes,
				fmt.Sprintf("%s %s", field.SrcName(), field.SrcType()))
		}
	}
	return strings.Join(nameTypes, ",")
}
