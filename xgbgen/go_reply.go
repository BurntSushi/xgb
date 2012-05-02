package main

func (r *Request) Define(c *Context) {
	c.Putln("// Request %s", r.SrcName())
	c.Putln("// size: %s", r.Size(c))
	c.Putln("")
	if r.Reply != nil {
		c.Putln("// Request reply for %s", r.SrcName())
		c.Putln("// size: %s", r.Reply.Size())
		c.Putln("type %s struct {", r.ReplyName())
		c.Putln("Sequence uint16")
		for _, field := range r.Reply.Fields {
			field.Define(c)
		}
		c.Putln("}")
		c.Putln("")
	}
}

