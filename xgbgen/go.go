package main
/*
	To the best of my ability, these are all of the Go specific formatting
	functions. If I've designed xgbgen correctly, this should be only the
	place that you change things to generate code for a new language.

	This file is organized as follows:

	* Imports and helper variables.
	* Manual type and name override maps.
	* Constants for tweaking various morphing functions.
	* Helper morphing functions.
	* Morphing functions for each "sub-unit."
	* Morphing functions for each "unit".
	* Morphing functions for collections of "units".
	
	Units can be thought of as the top-level elements in an XML protocol
	description file. Namely, structs, xidtypes, imports, enums, unions, etc.
	Collections of units are simply "all of the UNIT in the XML file."
	Sub-units can be thought of as recurring bits like struct contents (which
	is used in events, replies, requests, errors, etc.) and expression
	evaluation.
*/

import (
	"log"
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
	"char": "byte",
}

// BaseTypeSizes should have precisely the same keys as in BaseTypeMap,
// and the values should correspond to the size of the type in bytes.
var BaseTypeSizes = map[string]uint{
	"CARD8": 1,
	"CARD16": 2,
	"CARD32": 4,
	"INT8": 1,
	"INT16": 2,
	"INT32": 4,
	"BYTE": 1,
	"BOOL": 1,
	"float": 4,
	"double": 8,
	"char": 1,
	"Id": 4,
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
// Constants for changing the semantics of morphing functions.
// These are mainly used to tweaking the writing of fields.
// Namely, reading/writing is not exactly the same across events,
// requests/replies and errors.
/******************************************************************************/
const (
	FieldsEvent = iota
	FieldsRequestReply
	FieldsError
)

/******************************************************************************/
// Helper functions that aide in morphing repetitive constructs.
// i.e., type and identifier names, etc.
/******************************************************************************/

// Morph changes every TYPE (not names) into something suitable
// for your language. It also handles adding suffixes like 'Event'
// and 'Union'. (A 'Union' suffix is used in Go because unions aren't
// supported at the language level.)
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
	return typ.Prefix(c) + splitAndTitle(t)
}

// Prefix searches the parsed XML for a type matching 'typ'.
// It then returns the appropriate prefix to be used in source code.
// Note that the core X protocol *is* a namespace, but does not have a prefix.
// Also note that you should probably check the BaseTypeMap and TypeMap
// before calling this function.
func (typ Type) Prefix(c *Context) string {
	// If this is xproto, quit. No prefixes needed.
	if c.xml.Header == "xproto" {
		return ""
	}

	// First check for the type in the current namespace.
	if c.xml.HasType(typ) {
		return strings.Title(c.xml.Header)
	}

	// Now check each of the imports...
	for _, imp := range c.xml.Imports {
		if imp.xml.Header != "xproto" && imp.xml.HasType(typ) {
			return strings.Title(imp.xml.Header)
		}
	}

	return ""
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
// Sub-unit morphing.
// Below are functions that morph sub-units. Like collections of fields,
// expressions, etc.
// Note that collections of fields can be used in three different contexts:
// definitions, reading from the wire and writing to the wire. Thus, there
// exists 'MorphDefine', 'MorphRead', 'MorphWrite' defined on Fields.
/******************************************************************************/
func (fields Fields) MorphDefine(c *Context) {
	for _, field := range fields {
		field.MorphDefine(c)
	}
}

func (field *Field) MorphDefine(c *Context) {
	// We omit 'pad' and 'exprfield'
	switch field.XMLName.Local {
	case "field":
		c.Putln("%s %s", field.Name.Morph(c), field.Type.Morph(c))
	case "list":
		c.Putln("%s []%s", field.Name.Morph(c), field.Type.Morph(c))
	case "localfield":
		c.Putln("%s %s", field.Name.Morph(c), field.Type.Morph(c))
	case "valueparam":
		c.Putln("%s %s", field.ValueMaskName.Morph(c),
			field.ValueMaskType.Morph(c))
		c.Putln("%s []%s", field.ValueListName.Morph(c),
			field.ValueMaskType.Morph(c))
	case "switch":
		field.Bitcases.MorphDefine(c)
	}
}

func (bitcases Bitcases) MorphDefine(c *Context) {
	for _, bitcase := range bitcases {
		bitcase.MorphDefine(c)
	}
}

func (bitcase *Bitcase) MorphDefine(c *Context) {
	bitcase.Fields.MorphDefine(c)
}

func (fields Fields) MorphRead(c *Context, kind int, evNoSeq bool) {
	var nextByte uint

	switch kind {
	case FieldsEvent:
		nextByte = 1
	}

	for _, field := range fields {
		nextByte = field.MorphRead(c, kind, nextByte)
		switch kind {
		case FieldsEvent:
			// Skip the sequence id
			if !evNoSeq && (nextByte == 2 || nextByte == 3) {
				nextByte = 4
			}
		}
	}
}

func (field *Field) MorphRead(c *Context, kind int, byt uint) uint {
	consumed := uint(0)
	switch field.XMLName.Local {
	case "pad":
		consumed = uint(field.Bytes)
	case "field":
		if field.Type == "ClientMessageData" {
			break
		}
		size := field.Type.Size(c)
		typ := field.Type.Morph(c)
		name := field.Name.Morph(c)
		_, isBase := BaseTypeMap[string(field.Type)]

		c.Put("v.%s = ", name)
		if !isBase {
			c.Put("%s(", typ)
		}
		switch size {
		case 1:	c.Put("buf[%d]", byt)
		case 2: c.Put("get16(buf[%d:])", byt)
		case 4: c.Put("get32(buf[%d:])", byt)
		case 8: c.Put("get64(buf[%d:])", byt)
		default:
			log.Fatalf("Unsupported field size '%d' for field '%s'.",
				size, field)
		}
		if !isBase {
			c.Put(")")
		}
		c.Putln("")

		consumed = size
	case "list":
		c.Putln("")
	}
	return byt + consumed
}

func (fields Fields) MorphWrite(c *Context, kind int) {
	var nextByte uint

	switch kind {
	case FieldsEvent:
		nextByte = 1
	}

	for _, field := range fields {
		nextByte = field.MorphWrite(c, kind, nextByte)
	}
}

func (field *Field) MorphWrite(c *Context, kind int, byt uint) uint {
	consumed := uint(0)
	switch field.XMLName.Local {
	case "pad":
		consumed = uint(field.Bytes)
	case "field":
		size := field.Type.Size(c)
		typ := field.Type.Morph(c)
		name := field.Name.Morph(c)
		switch size {
		case 1:
			c.Putln("v.%s = %s(buf[%d])", name, typ, byt)
		case 2:
			c.Putln("v.%s = %s(get16(buf[%d:]))", name, typ, byt)
		case 4:
			c.Putln("v.%s = %s(get32(buf[%d:]))", name, typ, byt)
		case 8:
			c.Putln("v.%s = %s(get64(buf[%d:]))", name, typ, byt)
		}
		consumed = size
	case "list":
		c.Putln("IDK")
	}
	return byt + consumed
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
	c.Putln("type %s %s", typedef.New.Morph(c), typedef.Old.Morph(c))
}

// Struct morphing.
func (strct *Struct) Morph(c *Context) {
	c.Putln("type %s struct {", strct.Name.Morph(c))
	strct.Fields.MorphDefine(c)
	c.Putln("}")
	c.Putln("\n")
}

// Union morphing.
func (union *Union) Morph(c *Context) {
	c.Putln("type %s struct {", union.Name.Morph(c))
	union.Fields.MorphDefine(c)
	c.Putln("}")
	c.Putln("\n")
}

// Request morphing.
func (request *Request) Morph(c *Context) {
}

// Event morphing.
func (ev *Event) Morph(c *Context) {
	name := ev.Name.Morph(c)

	c.Putln("const %s = %d", name, ev.Number)
	c.Putln("")
	c.Putln("type %sEvent struct {", name)
	ev.Fields.MorphDefine(c)
	c.Putln("}")
	c.Putln("")
	c.Putln("func New%s(buf []byte) %sEvent {", name, name)
	c.Putln("var v %sEvent", name)
	ev.Fields.MorphRead(c, FieldsEvent, ev.NoSequence)
	c.Putln("return v")
	c.Putln("}")
	c.Putln("")
	c.Putln("func (err %sEvent) ImplementsEvent() { }", name)
	c.Putln("")
	c.Putln("func (ev %sEvent) Bytes() []byte {", name)
	// ev.Fields.MorphWrite(c, FieldsEvent) 
	c.Putln("}")
	c.Putln("")
	c.Putln("func init() {")
	c.Putln("newEventFuncs[%d] = New%s", ev.Number, name)
	c.Putln("}")
	c.Putln("")
}

// EventCopy morphing.
func (evcopy *EventCopy) Morph(c *Context) {
	oldName, newName := evcopy.Ref.Morph(c), evcopy.Name.Morph(c)

	c.Putln("const %s = %d", newName, evcopy.Number)
	c.Putln("")
	c.Putln("type %sEvent %sEvent", newName, oldName)
	c.Putln("")
	c.Putln("func New%s(buf []byte) %sEvent {", newName, newName)
	c.Putln("return (%sEvent)(New%s(buf))", newName, oldName)
	c.Putln("}")
	c.Putln("")
	c.Putln("func (err %sEvent) ImplementsEvent() { }", newName)
	c.Putln("")
	c.Putln("func (ev %sEvent) Bytes() []byte {", newName)
	c.Putln("return (%sEvent)(ev).Bytes()", oldName)
	c.Putln("}")
	c.Putln("")
	c.Putln("func init() {")
	c.Putln("newEventFuncs[%d] = New%s", evcopy.Number, newName)
	c.Putln("}")
	c.Putln("")
}

// Error morphing.
func (err *Error) Morph(c *Context) {
}

// ErrorCopy morphing.
func (errcopy *ErrorCopy) Morph(c *Context) {
	oldName, newName := errcopy.Ref.Morph(c), errcopy.Name.Morph(c)

	c.Putln("const Bad%s = %d", newName, errcopy.Number)
	c.Putln("")
	c.Putln("type %sError %sError", newName, oldName)
	c.Putln("")
	c.Putln("func New%sError(buf []byte) %sError {", newName, newName)
	c.Putln("return (%sError)(New%sError(buf))", newName, oldName)
	c.Putln("}")
	c.Putln("")
	c.Putln("func (err %sError) ImplementsError() { }", newName)
	c.Putln("")
	c.Putln("func (err %sError) Bytes() []byte {", newName)
	c.Putln("return (%sError)(err).Bytes()", oldName)
	c.Putln("}")
	c.Putln("")
	c.Putln("func init() {")
	c.Putln("newErrorFuncs[%d] = New%sError", errcopy.Number, newName)
	c.Putln("}")
	c.Putln("")
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

