package main

import (
	"fmt"
	"log"
	"unicode"
)

type Protocol struct {
	Name         string
	ExtXName     string
	ExtName      string
	MajorVersion string
	MinorVersion string

	Imports  []*Protocol
	Types    []Type
	Requests []*Request
}

// Initialize traverses all structures, looks for 'Translation' type,
// and looks up the real type in the namespace. It also sets the source
// name for all relevant fields/structures.
// This is necessary because we don't traverse the XML in order initially.
func (p *Protocol) Initialize() {
	for _, typ := range p.Types {
		typ.Initialize(p)
	}
	for _, req := range p.Requests {
		req.Initialize(p)
	}
}

type Request struct {
	srcName string
	xmlName string
	Opcode  int
	Combine bool
	Fields  []Field
	Reply   *Reply
}

func (r *Request) Initialize(p *Protocol) {
	r.srcName = SrcName(r.xmlName)
	if r.Reply != nil {
		r.Reply.Initialize(p)
	}
	for _, field := range r.Fields {
		field.Initialize(p)
	}
}

func (r *Request) SrcName() string {
	return r.srcName
}

func (r *Request) XmlName() string {
	return r.xmlName
}

func (r *Request) ReplyName() string {
	if r.Reply == nil {
		log.Panicf("Cannot call 'ReplyName' on request %s, which has no reply.",
			r.SrcName())
	}
	name := r.SrcName()
	lower := string(unicode.ToLower(rune(name[0]))) + name[1:]
	return fmt.Sprintf("%sReply", lower)
}

func (r *Request) ReplyTypeName() string {
	if r.Reply == nil {
		log.Panicf("Cannot call 'ReplyName' on request %s, which has no reply.",
			r.SrcName())
	}
	return fmt.Sprintf("%sReply", r.SrcName())
}

func (r *Request) ReqName() string {
	name := r.SrcName()
	lower := string(unicode.ToLower(rune(name[0]))) + name[1:]
	return fmt.Sprintf("%sRequest", lower)
}

func (r *Request) CookieName() string {
	return fmt.Sprintf("%sCookie", r.SrcName())
}

// Size for Request needs a context.
// Namely, if this is an extension, we need to account for *four* bytes
// of a header (extension opcode, request opcode, and the sequence number).
// If it's a core protocol request, then we only account for *three*
// bytes of the header (remove the extension opcode).
func (r *Request) Size(c *Context) Size {
	size := newFixedSize(0)

	if c.protocol.Name == "xproto" {
		size = size.Add(newFixedSize(3))
	} else {
		size = size.Add(newFixedSize(4))
	}

	for _, field := range r.Fields {
		switch field.(type) {
		case *LocalField:
			continue
		case *SingleField:
			// mofos!!!
			if r.SrcName() == "ConfigureWindow" &&
				field.SrcName() == "ValueMask" {

				continue
			}
			size = size.Add(field.Size())
		default:
			size = size.Add(field.Size())
		}
	}
	return newExpressionSize(&Padding{
		Expr: size.Expression,
	})
}

type Reply struct {
	Fields []Field
}

func (r *Reply) Size() Size {
	size := newFixedSize(0)

	// Account for reply discriminant, sequence number and reply length
	size = size.Add(newFixedSize(7))

	for _, field := range r.Fields {
		size = size.Add(field.Size())
	}
	return size
}

func (r *Reply) Initialize(p *Protocol) {
	for _, field := range r.Fields {
		field.Initialize(p)
	}
}
