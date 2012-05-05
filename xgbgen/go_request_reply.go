package main

import (
	"fmt"
	"strings"
)

func (r *Request) Define(c *Context) {
	c.Putln("// Request %s", r.SrcName())
	c.Putln("// size: %s", r.Size(c))
	c.Putln("type %s cookie", r.CookieName())
	if r.Reply != nil {
		c.Putln("func (c *Conn) %s(%s) %s {",
			r.SrcName(), r.ParamNameTypes(), r.CookieName())
		c.Putln("cookie := c.newCookie(true, true)")
		c.Putln("c.newRequest(%s(%s), cookie)", r.ReqName(), r.ParamNames())
		c.Putln("return %s(cookie)", r.CookieName())
		c.Putln("}")
		c.Putln("")

		c.Putln("func (c *Conn) %sUnchecked(%s) %s {",
			r.SrcName(), r.ParamNameTypes(), r.CookieName())
		c.Putln("cookie := c.newCookie(false, true)")
		c.Putln("c.newRequest(%s(%s), cookie)", r.ReqName(), r.ParamNames())
		c.Putln("return %s(cookie)", r.CookieName())
		c.Putln("}")
		c.Putln("")

		r.ReadReply(c)
	} else {
		c.Putln("// Write request to wire for %s", r.SrcName())
		c.Putln("func (c *Conn) %s(%s) %s {",
			r.SrcName(), r.ParamNameTypes(), r.CookieName())
		c.Putln("cookie := c.newCookie(false, false)")
		c.Putln("c.newRequest(%s(%s), cookie)", r.ReqName(), r.ParamNames())
		c.Putln("return %s(cookie)", r.CookieName())
		c.Putln("}")
		c.Putln("")

		c.Putln("func (c *Conn) %sChecked(%s) %s {",
			r.SrcName(), r.ParamNameTypes(), r.CookieName())
		c.Putln("cookie := c.newCookie(true, false)")
		c.Putln("c.newRequest(%s(%s), cookie)", r.ReqName(), r.ParamNames())
		c.Putln("return %s(cookie)", r.CookieName())
		c.Putln("}")
		c.Putln("")
	}

	r.WriteRequest(c)
}

func (r *Request) ReadReply(c *Context) {
	c.Putln("// Request reply for %s", r.SrcName())
	c.Putln("// size: %s", r.Reply.Size())
	c.Putln("type %s struct {", r.ReplyTypeName())
	c.Putln("Sequence uint16")
	c.Putln("Length uint32")
	for _, field := range r.Reply.Fields {
		field.Define(c)
	}
	c.Putln("}")
	c.Putln("")

	c.Putln("// Waits and reads reply data from request %s", r.SrcName())
	c.Putln("func (cook %s) Reply() (*%s, error) {",
		r.CookieName(), r.ReplyTypeName())
		c.Putln("buf, err := cookie(cook).reply()")
		c.Putln("if err != nil {")
		c.Putln("return nil, err")
		c.Putln("}")
		c.Putln("return %s(buf), nil", r.ReplyName())
	c.Putln("}")
	c.Putln("")

	c.Putln("// Read reply into structure from buffer for %s", r.SrcName())
	c.Putln("func %s(buf []byte) *%s {",
		r.ReplyName(), r.ReplyTypeName())
	c.Putln("v := new(%s)", r.ReplyTypeName())
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
	c.Putln("return v")
	c.Putln("}")
	c.Putln("")
}

func (r *Request) WriteRequest(c *Context) {
	c.Putln("// Write request to wire for %s", r.SrcName())
	c.Putln("func %s(%s) []byte {", r.ReqName(), r.ParamNameTypes())
	c.Putln("size := %s", r.Size(c))
	c.Putln("b := 0")
	c.Putln("buf := make([]byte, size)")
	c.Putln("")
	c.Putln("buf[b] = %d // request opcode", r.Opcode)
	c.Putln("b += 1")
	c.Putln("")
	if strings.ToLower(c.protocol.Name) != "xproto" {
		c.Putln("buf[b] = c.extensions[\"%s\"]",
			strings.ToUpper(c.protocol.ExtXName))
		c.Putln("b += 1")
		c.Putln("")
	}
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
	c.Putln("return buf")
	c.Putln("}")
	c.Putln("")
}

func (r *Request) ParamNames() string {
	names := make([]string, 0, len(r.Fields))
	for _, field := range r.Fields {
		switch f := field.(type) {
		case *ValueField:
			// mofos...
			if r.SrcName() != "ConfigureWindow" {
				names = append(names, f.MaskName)
			}
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
