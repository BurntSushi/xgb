package main
/*
	To the best of my ability, these are all of the Go specific formatting
	functions. If I've designed xgbgen correctly, this should be only the
	place that you change things to generate code for a new language.

	This file is organized as follows:

	* Imports and helper variables.
	* Manual type and name override maps.
	* Helper morphing functions.
	* Morphing functions for each "unit".
	* Morphing functions for collections of "units".
*/

import (
	"strings"
)

/******************************************************************************/
// Manual type and name overrides.
/******************************************************************************/

// BaseTypeMap is a map from X base types to Go types.
// X base types should correspond to the smallest set of X types
// that can be used to rewrite ALL X types in terms of Go types.
// That is, if you remove any of the following types, at least one
// XML protocol description will produce an invalid Go program.
// The types on the left *never* show themselves in the source.
var BaseTypeMap = map[string]string{
	"CARD8": "byte",
	"CARD16": "uint16",
	"CARD32": "uint32",
	"INT8": "int8",
	"INT16": "int16",
	"INT32": "int32",
	"BYTE": "byte",
	"BOOL": "bool",
	"float": "float64",
	"double": "float64",
}

// TypeMap is a map from types in the XML to type names that is used
// in the functions that follow. Basically, every occurrence of the key
// type is replaced with the value type.
var TypeMap = map[string]string{
	"VISUALTYPE": "VisualInfo",
	"DEPTH": "DepthInfo",
	"SCREEN": "ScreenInfo",
	"Setup": "SetupInfo",
}

// NameMap is the same as TypeMap, but for names.
var NameMap = map[string]string{ }

/******************************************************************************/
// Helper functions that aide in morphing repetive constructs.
// i.e., "structure contents", expressions, type and identifier names, etc.
/******************************************************************************/

// Morph changes every TYPE (not names) into something suitable
// for your language.
func (typ Type) Morph(c *Context) string {
	t := string(typ)

	// If this is a base type, then write the raw Go type.
	if newt, ok := BaseTypeMap[t]; ok {
		return newt
	}

	// If it's in the type map, use that translation.
	if newt, ok := TypeMap[t]; ok {
		return newt
	}

	// If it's a resource type, just use 'Id'.
	if c.xml.IsResource(typ) {
		return "Id"
	}

	// If there's a namespace to this type, just use it and be done.
	if colon := strings.Index(t, ":"); colon > -1 {
		namespace := t[:colon]
		rest := t[colon+1:]
		return splitAndTitle(namespace) + splitAndTitle(rest)
	}

	// Since there is no namespace, we need to look for a namespace
	// in the current context.
	return c.TypePrefix(typ) + splitAndTitle(t)
}

// Morph changes every identifier (NOT type) into something suitable
// for your language.
func (name Name) Morph(c *Context) string {
	n := string(name)

	// If it's in the name map, use that translation.
	if newn, ok := NameMap[n]; ok {
		return newn
	}

	return splitAndTitle(n)
}

/******************************************************************************/
// Per element morphing.
// Below are functions that morph a single unit.
/******************************************************************************/

// Import morphing.
func (imp *Import) Morph(c *Context) {
	c.Putln("// import \"%s\"", imp.Name)
}

// Enum morphing.
func (enum *Enum) Morph(c *Context) {
	c.Putln("const (")
	for _, item := range enum.Items {
		c.Putln("%s%s = %d", enum.Name.Morph(c), item.Name.Morph(c),
			item.Expr.Eval())
	}
	c.Putln(")\n")
}

// Xid morphing.
func (xid *Xid) Morph(c *Context) {
	// Don't emit anything for xid types for now.
	// We're going to force them all to simply be 'Id'
	// to avoid excessive type converting.
	// c.Putln("type %s Id", xid.Name.Morph(c)) 
}

// TypeDef morphing.
func (typedef *TypeDef) Morph(c *Context) {
	c.Putln("type %s %s", typedef.Old.Morph(c), typedef.New.Morph(c))
}

// Struct morphing.
func (strct *Struct) Morph(c *Context) {
}

// Union morphing.
func (union *Union) Morph(c *Context) {
}

// Request morphing.
func (request *Request) Morph(c *Context) {
}

// Event morphing.
func (ev *Event) Morph(c *Context) {
}

// EventCopy morphing.
func (evcopy *EventCopy) Morph(c *Context) {
}

// Error morphing.
func (err *Error) Morph(c *Context) {
}

// ErrorCopy morphing.
func (errcopy *ErrorCopy) Morph(c *Context) {
}

/******************************************************************************/
// Collection morphing.
// Below are functions that morph a collections of units.
// Most of these can probably remain unchanged, but they are useful if you
// need to group all of some "unit" in a single block or something.
/******************************************************************************/
func (imports Imports) Morph(c *Context) {
	if len(imports) == 0 {
		return
	}

	c.Putln("// Imports are not required for XGB since everything is in")
	c.Putln("// a single package. Still these may be useful for ")
	c.Putln("// reference purposes.")
	for _, imp := range imports {
		imp.Morph(c)
	}
}

func (enums Enums) Morph(c *Context) {
	c.Putln("// Enums\n")
	for _, enum := range enums {
		enum.Morph(c)
	}
}

func (xids Xids) Morph(c *Context) {
	c.Putln("// Xids\n")
	for _, xid := range xids {
		xid.Morph(c)
	}
}

func (typedefs TypeDefs) Morph(c *Context) {
	c.Putln("// TypeDefs\n")
	for _, typedef := range typedefs {
		typedef.Morph(c)
	}
}

func (strct Structs) Morph(c *Context) {
	c.Putln("// Structs\n")
	for _, typedef := range strct {
		typedef.Morph(c)
	}
}

func (union Unions) Morph(c *Context) {
	c.Putln("// Unions\n")
	for _, typedef := range union {
		typedef.Morph(c)
	}
}

func (request Requests) Morph(c *Context) {
	c.Putln("// Requests\n")
	for _, typedef := range request {
		typedef.Morph(c)
	}
}

func (event Events) Morph(c *Context) {
	c.Putln("// Events\n")
	for _, typedef := range event {
		typedef.Morph(c)
	}
}

func (evcopy EventCopies) Morph(c *Context) {
	c.Putln("// Event Copies\n")
	for _, typedef := range evcopy {
		typedef.Morph(c)
	}
}

func (err Errors) Morph(c *Context) {
	c.Putln("// Errors\n")
	for _, typedef := range err {
		typedef.Morph(c)
	}
}

func (errcopy ErrorCopies) Morph(c *Context) {
	c.Putln("// Error copies\n")
	for _, typedef := range errcopy {
		typedef.Morph(c)
	}
}

