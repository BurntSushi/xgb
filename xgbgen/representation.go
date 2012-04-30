package main

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

type Reply struct {
	Fields []Field
}

func (r *Reply) Initialize(p *Protocol) {
	for _, field := range r.Fields {
		field.Initialize(p)
	}
}
