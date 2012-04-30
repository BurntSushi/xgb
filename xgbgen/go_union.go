package main

// Union types
func (u *Union) Define(c *Context) {
	c.Putln("// Union definition %s", u.SrcName())
}

func (u *Union) Read(c *Context, prefix string) {
	c.Putln("// Union read %s", u.SrcName())
}

func (u *Union) Write(c *Context, prefix string) {
	c.Putln("// Union write %s", u.SrcName())
}

