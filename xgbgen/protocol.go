package main

import (
	"strings"
)

// Protocol is a type that encapsulates all information about one
// particular XML file. It also contains links to other protocol types
// if this protocol imports other other extensions. The import relationship
// is recursive.
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

// isExt returns true if this protocol is an extension.
// i.e., it's name isn't "xproto".
func (p *Protocol) isExt() bool {
	return strings.ToLower(p.Name) == "xproto"
}

