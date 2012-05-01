package main

// Union types
func (u *Union) Define(c *Context) {
	c.Putln("// Union definition %s", u.SrcName())
	c.Putln("// Note that to *create* a Union, you should *never* create")
	c.Putln("// this struct directly (unless you know what you're doing).")
	c.Putln("// Instead use one of the following constructors for '%s':",
		u.SrcName())
	for _, field := range u.Fields {
		c.Putln("//     New%s%s(%s %s) %s", u.SrcName(), field.SrcName(),
			field.SrcName(), field.SrcType(), u.SrcName())
	}

	c.Putln("type %s struct {", u.SrcName())
	for _, field := range u.Fields {
		field.Define(c)
	}
	c.Putln("}")
	c.Putln("")

	// Write functions for each field that create instances of this
	// union using the corresponding field.
	u.New(c)

	// Write function that reads bytes and produces this union.
	u.Read(c)

	// Write function that reads bytes and produces a list of this union.
	u.ReadList(c)

	// Write function that writes bytes given this union.
	u.Write(c)

	// Write function that writes a list of this union.
	u.WriteList(c)

	// Write function that computes the size of a list of these unions.
	u.WriteListSize(c)
}

func (u *Union) New(c *Context) {
	for _, field := range u.Fields {
		c.Putln("// Union constructor for %s for field %s.",
			u.SrcName(), field.SrcName())
		c.Putln("func New%s%s(%s %s) %s {",
			u.SrcName(), field.SrcName(), field.SrcName(),
			field.SrcType(), u.SrcName())
		c.Putln("var b int")
		c.Putln("buf := make([]byte, %s)", u.Size())
		c.Putln("")
		field.Write(c)
		c.Putln("")
		c.Putln("// Create the Union type")
		c.Putln("v := %s{}", u.SrcName())
		c.Putln("")
		c.Putln("// Now copy buf into all fields")
		c.Putln("")
		for _, field2 := range u.Fields {
			c.Putln("b = 0 // always read the same bytes")
			field2.Read(c)
			c.Putln("")
		}
		c.Putln("return v")
		c.Putln("}")
		c.Putln("")
	}
}

func (u *Union) Read(c *Context) {
	c.Putln("// Union read %s", u.SrcName())
	c.Putln("func Read%s(buf []byte, v *%s) int {", u.SrcName(), u.SrcName())
	c.Putln("var b int")
	c.Putln("")
	for _, field := range u.Fields {
		c.Putln("b = 0 // re-read the same bytes")
		field.Read(c)
		c.Putln("")
	}
	c.Putln("return %s", u.Size())
	c.Putln("}")
	c.Putln("")
}

func (u *Union) ReadList(c *Context) {
	c.Putln("// Union list read %s", u.SrcName())
	c.Putln("func Read%sList(buf []byte, dest []%s) int {",
		u.SrcName(), u.SrcName())
	c.Putln("b := 0")
	c.Putln("for i := 0; i < len(dest); i++ {")
	c.Putln("dest[i] = %s{}", u.SrcName())
	c.Putln("b += Read%s(buf[b:], &dest[i])", u.SrcName())
	c.Putln("}")
	c.Putln("return pad(b)")
	c.Putln("}")
	c.Putln("")
}

// This is a bit tricky since writing from a Union implies that only
// the data inside ONE of the elements is actually written.
// However, we only currently support unions where every field has the
// *same* *fixed* size. Thus, we make sure to always read bytes into
// every field which allows us to simply pick the first field and write it.
func (u *Union) Write(c *Context) {
	c.Putln("// Union write %s", u.SrcName())
	c.Putln("// Each field in a union must contain the same data.")
	c.Putln("// So simply pick the first field and write that to the wire.")
	c.Putln("func (v %s) Bytes() []byte {", u.SrcName())
	c.Putln("buf := make([]byte, %s)", u.Size().Reduce("v.", ""))
	c.Putln("b := 0")
	c.Putln("")
	u.Fields[0].Write(c)
	c.Putln("return buf")
	c.Putln("}")
	c.Putln("")
}

func (u *Union) WriteList(c *Context) {
	c.Putln("// Union list write %s", u.SrcName())
	c.Putln("func %sListBytes(buf []byte, list []%s) int {",
		u.SrcName(), u.SrcName())
	c.Putln("b := 0")
	c.Putln("var unionBytes []byte")
	c.Putln("for _, item := range list {")
	c.Putln("unionBytes = item.Bytes()")
	c.Putln("copy(buf[b:], len(unionBytes))")
	c.Putln("b += pad(len(unionBytes))")
	c.Putln("}")
	c.Putln("return b")
	c.Putln("}")
	c.Putln("")
}

func (u *Union) WriteListSize(c *Context) {
	c.Putln("// Union list size %s", u.SrcName())
	c.Putln("func %sListSize(list []%s) int {", u.SrcName(), u.SrcName())
	c.Putln("size := 0")
	c.Putln("for _, item := range list {")
	c.Putln("size += %s", u.Size().Reduce("item.", ""))
	c.Putln("}")
	c.Putln("return size")
	c.Putln("}")
	c.Putln("")
}

