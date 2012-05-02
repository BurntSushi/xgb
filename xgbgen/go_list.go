package main

import (
	"fmt"
	"log"
)

// List fields
func (f *ListField) Define(c *Context) {
	c.Putln("%s []%s // size: %s",
		f.SrcName(), f.Type.SrcName(), f.Size())
}

func (f *ListField) Read(c *Context) {
	switch t := f.Type.(type) {
	case *Resource:
		length := f.LengthExpr.Reduce("v.", "")
		c.Putln("v.%s = make([]Id, %s)", f.SrcName(), length)
		c.Putln("for i := 0; i < %s; i++ {", length)
		ReadSimpleSingleField(c, fmt.Sprintf("v.%s[i]", f.SrcName()), t)
		c.Putln("}")
		c.Putln("b = pad(b)")
	case *Base:
		length := f.LengthExpr.Reduce("v.", "")
		c.Putln("v.%s = make([]%s, %s)", f.SrcName(), t.SrcName(), length)
		if t.SrcName() == "byte" {
			c.Putln("copy(v.%s[:%s], buf[b:])", f.SrcName(), length)
			c.Putln("b += pad(%s)", length)
		} else {
			c.Putln("for i := 0; i < %s; i++ {", length)
			ReadSimpleSingleField(c, fmt.Sprintf("v.%s[i]", f.SrcName()), t)
			c.Putln("}")
			c.Putln("b = pad(b)")
		}
	case *Union:
		c.Putln("v.%s = make([]%s, %s)",
			f.SrcName(), t.SrcName(), f.LengthExpr.Reduce("v.", ""))
		c.Putln("b += Read%sList(buf[b:], v.%s)", t.SrcName(), f.SrcName())
	case *Struct:
		c.Putln("v.%s = make([]%s, %s)",
			f.SrcName(), t.SrcName(), f.LengthExpr.Reduce("v.", ""))
		c.Putln("b += Read%sList(buf[b:], v.%s)", t.SrcName(), f.SrcName())
	default:
		log.Panicf("Cannot read list field '%s' with %T type.",
			f.XmlName(), f.Type)
	}
}

func (f *ListField) Write(c *Context) {
	switch t := f.Type.(type) {
	case *Resource:
		length := f.LengthExpr.Reduce("v.", "")
		c.Putln("for i := 0; i < %s; i++", length)
		WriteSimpleSingleField(c, fmt.Sprintf("v.%s[i]", f.SrcName()), t)
		c.Putln("}")
		c.Putln("b = pad(b)")
	case *Base:
		length := f.LengthExpr.Reduce("v.", "")
		if t.SrcName() == "byte" {
			c.Putln("copy(buf[b:], v.%s[:%s])", f.SrcName(), length)
			c.Putln("b += pad(%s)", length)
		} else {
			c.Putln("for i := 0; i < %s; i++ {", length)
			WriteSimpleSingleField(c, fmt.Sprintf("v.%s[i]", f.SrcName()), t)
			c.Putln("}")
			c.Putln("b = pad(b)")
		}
	case *Union:
		c.Putln("b += %sListBytes(buf[b:], v.%s)", t.SrcName(), f.SrcName())
	case *Struct:
		c.Putln("b += %sListBytes(buf[b:], v.%s)", t.SrcName(), f.SrcName())
	default:
		log.Panicf("Cannot read list field '%s' with %T type.",
			f.XmlName(), f.Type)
	}
}

