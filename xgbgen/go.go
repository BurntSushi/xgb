package main

import (
	"fmt"
	"log"
)

// xgbResourceIdName is the name of the type used for all resource identifiers.
// As of right now, it needs to be declared somewhere manually.
var xgbGenResourceIdName = "Id"

// BaseTypeMap is a map from X base types to Go types.
// X base types should correspond to the smallest set of X types
// that can be used to rewrite ALL X types in terms of Go types.
// That is, if you remove any of the following types, at least one
// XML protocol description will produce an invalid Go program.
// The types on the left *never* show themselves in the source.
var BaseTypeMap = map[string]string{
	"CARD8":  "byte",
	"CARD16": "uint16",
	"CARD32": "uint32",
	"INT8":   "int8",
	"INT16":  "int16",
	"INT32":  "int32",
	"BYTE":   "byte",
	"BOOL":   "bool",
	"float":  "float64",
	"double": "float64",
	"char":   "byte",
	"void":   "byte",
	"Id":     "Id",
}

// BaseTypeSizes should have precisely the same keys as in BaseTypeMap,
// and the values should correspond to the size of the type in bytes.
var BaseTypeSizes = map[string]uint{
	"CARD8":  1,
	"CARD16": 2,
	"CARD32": 4,
	"INT8":   1,
	"INT16":  2,
	"INT32":  4,
	"BYTE":   1,
	"BOOL":   1,
	"float":  4,
	"double": 8,
	"char":   1,
	"void":   1,
	"Id":     4,
}

// TypeMap is a map from types in the XML to type names that is used
// in the functions that follow. Basically, every occurrence of the key
// type is replaced with the value type.
var TypeMap = map[string]string{
	"VISUALTYPE": "VisualInfo",
	"DEPTH":      "DepthInfo",
	"SCREEN":     "ScreenInfo",
	"Setup":      "SetupInfo",
}

// NameMap is the same as TypeMap, but for names.
var NameMap = map[string]string{}

// Reading, writing and defining...

// Base types
func (b *Base) Define(c *Context) {
	c.Putln("// Skipping definition for base type '%s'", SrcName(b.XmlName()))
	c.Putln("")
}

// Enum types
func (enum *Enum) Define(c *Context) {
	c.Putln("const (")
	for _, item := range enum.Items {
		c.Putln("%s%s = %d", enum.SrcName(), item.srcName, item.Expr.Eval())
	}
	c.Putln(")")
	c.Putln("")
}

// Resource types
func (res *Resource) Define(c *Context) {
	c.Putln("// Skipping resource definition of '%s'", SrcName(res.XmlName()))
	c.Putln("")
}

// TypeDef types
func (td *TypeDef) Define(c *Context) {
	c.Putln("type %s %s", td.srcName, td.Old.SrcName())
	c.Putln("")
}

// Struct types
func (s *Struct) Define(c *Context) {
	c.Putln("// '%s' struct definition", s.SrcName())
	c.Putln("// Size: %s", s.Size())
	c.Putln("type %s struct {", s.SrcName())
	for _, field := range s.Fields {
		field.Define(c)
	}
	c.Putln("}")
	c.Putln("")

	// Write function that reads bytes and produces this struct.
	s.Read(c)

	// Write function that reads a list of this structs.
	s.ReadList(c)

	// Write function that writes bytes given this struct.
	s.Write(c)

	// Write function that writes a list of this struct.
	s.WriteList(c)
}

// Read for a struct creates a function 'NewStructName' that takes a byte
// slice and produces TWO values: an instance of 'StructName' and the number
// of bytes read from the byte slice.
// 'NewStructName' should only be used to read raw reply data from the wire.
func (s *Struct) Read(c *Context) {
	c.Putln("// Struct read %s", s.SrcName())
	c.Putln("func New%s(buf []byte) (%s, int) {", s.SrcName(), s.SrcName())

	c.Putln("v := %s{}", s.SrcName())
	c.Putln("b := 0")
	c.Putln("consumed := 0")
	c.Putln("consumed = 0 + consumed // no-op") // dirty hack for a no-op
	c.Putln("")
	for _, field := range s.Fields {
		field.Read(c)
	}
	c.Putln("return v, b")

	c.Putln("}")
	c.Putln("")
}

// ReadList for a struct creates a function 'ReadStructNameList' that takes
// a byte slice and a length and produces TWO values: an slice of StructName 
// and the number of bytes read from the byte slice.
func (s *Struct) ReadList(c *Context) {
	c.Putln("// Struct list read %s", s.SrcName())
	c.Putln("func Read%sList(buf []byte, length int) ([]%s, int) {",
		s.SrcName(), s.SrcName())

	c.Putln("v := make([]%s, length)", s.SrcName())
	c.Putln("b := 0")
	c.Putln("consumed := 0")
	c.Putln("consumed = 0 + consumed // no-op") // dirty hack for a no-op
	c.Putln("for i := 0; i < length; i++ {")
	c.Putln("v[i], consumed = New%s(buf[b:])", s.SrcName())
	c.Putln("b += consumed")
	c.Putln("}")

	c.Putln("return v, pad(b)")

	c.Putln("}")
	c.Putln("")
}

func (s *Struct) Write(c *Context) {
	c.Putln("// Struct write %s", s.SrcName())
	c.Putln("")
}

func (s *Struct) WriteList(c *Context) {
	c.Putln("// Write struct list %s", s.SrcName())
	c.Putln("")
}

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

// Event types
func (e *Event) Define(c *Context) {
	c.Putln("// Event definition %s (%d)", e.SrcName(), e.Number)
}

func (e *Event) Read(c *Context, prefix string) {
	c.Putln("// Event read %s", e.SrcName())
}

func (e *Event) Write(c *Context, prefix string) {
	c.Putln("// Event write %s", e.SrcName())
}

// EventCopy types
func (e *EventCopy) Define(c *Context) {
	c.Putln("// EventCopy definition %s (%d)", e.SrcName(), e.Number)
	c.Putln("")
	c.Putln("const %s = %d", e.SrcName(), e.Number)
	c.Putln("")
	c.Putln("type %s %s", e.EvType(), e.Old.(*Event).EvType())
	c.Putln("")
	c.Putln("func New%s(buf []byte) %s {", e.SrcName(), e.EvType())
	c.Putln("return (%s)(New%s(buf))", e.EvType(), e.Old.SrcName())
	c.Putln("}")
	c.Putln("")
	c.Putln("func (ev %s) ImplementsEvent() { }", e.EvType())
	c.Putln("")
	c.Putln("func (ev %s) Bytes() []byte {", e.EvType())
	c.Putln("return (%s)(ev).Bytes()", e.Old.(*Event).EvType())
	c.Putln("}")
	c.Putln("")
	c.Putln("func init() {")
	c.Putln("newEventFuncs[%d] = New%s", e.Number, e.SrcName())
	c.Putln("}")
	c.Putln("")
}

// Error types
func (e *Error) Define(c *Context) {
	c.Putln("// Error definition %s (%d)", e.SrcName(), e.Number)
	c.Putln("")
}

func (e *Error) Read(c *Context, prefix string) {
	c.Putln("// Error read %s", e.SrcName())
}

func (e *Error) Write(c *Context, prefix string) {
	c.Putln("// Error write %s", e.SrcName())
}

// ErrorCopy types
func (e *ErrorCopy) Define(c *Context) {
	c.Putln("// ErrorCopy definition %s (%d)", e.SrcName(), e.Number)
	c.Putln("")
	c.Putln("const %s = %d", e.ErrConst(), e.Number)
	c.Putln("")
	c.Putln("type %s %s", e.ErrType(), e.Old.(*Error).ErrType())
	c.Putln("")
	c.Putln("func New%s(buf []byte) %s {", e.SrcName(), e.ErrType())
	c.Putln("return (%s)(New%s(buf))", e.ErrType(), e.Old.SrcName())
	c.Putln("}")
	c.Putln("")
	c.Putln("func (err %s) ImplementsError() { }", e.ErrType())
	c.Putln("")
	c.Putln("func (err %s) Bytes() []byte {", e.ErrType())
	c.Putln("return (%s)(err).Bytes()", e.Old.(*Error).ErrType())
	c.Putln("}")
	c.Putln("")
	c.Putln("func init() {")
	c.Putln("newErrorFuncs[%d] = New%s", e.Number, e.SrcName())
	c.Putln("}")
	c.Putln("")
}

// Field definitions, reads and writes.

// Pad fields
func (f *PadField) Define(c *Context) {
	c.Putln("// padding: %d bytes", f.Bytes)
}

func (f *PadField) Read(c *Context) {
	c.Putln("b += %s // padding", f.Size())
	c.Putln("")
}

// Single fields
func (f *SingleField) Define(c *Context) {
	c.Putln("%s %s", f.SrcName(), f.Type.SrcName())
}

func ReadSimpleSingleField(c *Context, name string, typ Type) {
	switch t := typ.(type) {
	case *Resource:
		c.Putln("%s = get32(buf[b:])", name)
	case *TypeDef:
		switch t.Size().Eval() {
		case 1:
			c.Putln("%s = %s(buf[b])", name, t.SrcName())
		case 2:
			c.Putln("%s = %s(get16(buf[b:]))", name, t.SrcName())
		case 4:
			c.Putln("%s = %s(get32(buf[b:]))", name, t.SrcName())
		case 8:
			c.Putln("%s = %s(get64(buf[b:]))", name, t.SrcName())
		}
	case *Base:
		var val string
		switch t.Size().Eval() {
		case 1:
			val = fmt.Sprintf("buf[b]")
		case 2:
			val = fmt.Sprintf("get16(buf[b:])")
		case 4:
			val = fmt.Sprintf("get32(buf[b:])")
		case 8:
			val = fmt.Sprintf("get64(buf[b:])")
		}

		// We need to convert base types if they aren't uintXX or byte
		ty := t.SrcName()
		if ty != "byte" && ty != "uint16" && ty != "uint32" && ty != "uint64" {
			val = fmt.Sprintf("%s(%s)", ty, val)
		}
		c.Putln("%s = %s", name, val)
	default:
		log.Fatalf("Cannot read field '%s' as a simple field with %T type.",
			name, typ)
	}

	c.Putln("b += %s", typ.Size())
}

func (f *SingleField) Read(c *Context) {
	switch t := f.Type.(type) {
	case *Resource:
		ReadSimpleSingleField(c, fmt.Sprintf("v.%s", f.SrcName()), t)
	case *TypeDef:
		ReadSimpleSingleField(c, fmt.Sprintf("v.%s", f.SrcName()), t)
	case *Base:
		ReadSimpleSingleField(c, fmt.Sprintf("v.%s", f.SrcName()), t)
	case *Struct:
		c.Putln("v.%s, consumed = New%s(buf[b:])", f.SrcName(), t.SrcName())
		c.Putln("b += consumed")
		c.Putln("")
	default:
		log.Fatalf("Cannot read field '%s' with %T type.", f.XmlName(), f.Type)
	}
}

// List fields
func (f *ListField) Define(c *Context) {
	c.Putln("%s []%s // length: %s",
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
		c.Putln("")
	case *Base:
		length := f.LengthExpr.Reduce("v.", "")
		c.Putln("v.%s = make([]%s, %s)", f.SrcName(), t.SrcName(), length)
		c.Putln("for i := 0; i < %s; i++ {", length)
		ReadSimpleSingleField(c, fmt.Sprintf("v.%s[i]", f.SrcName()), t)
		c.Putln("}")
		c.Putln("")
	case *Struct:
		c.Putln("v.%s, consumed = Read%sList(buf[b:], %s)",
			f.SrcName(), t.SrcName(), f.LengthExpr.Reduce("v.", ""))
		c.Putln("b += consumed")
		c.Putln("")
	default:
		log.Fatalf("Cannot read list field '%s' with %T type.",
			f.XmlName(), f.Type)
	}
}

// Local fields
func (f *LocalField) Define(c *Context) {
	c.Putln("// local field: %s %s", f.SrcName(), f.Type.SrcName())
}

func (f *LocalField) Read(c *Context) {
	c.Putln("// reading local field: %s (%s) :: %s",
		f.SrcName(), f.Size(), f.Type.SrcName())
}

// Expr fields
func (f *ExprField) Define(c *Context) {
	c.Putln("// expression field: %s %s (%s)",
		f.SrcName(), f.Type.SrcName(), f.Expr)
}

func (f *ExprField) Read(c *Context) {
	c.Putln("// reading expression field: %s (%s) (%s) :: %s",
		f.SrcName(), f.Size(), f.Expr, f.Type.SrcName())
}

// Value field
func (f *ValueField) Define(c *Context) {
	c.Putln("// valueparam field: type: %s, mask name: %s, list name: %s",
		f.MaskType.SrcName(), f.MaskName, f.ListName)
}

func (f *ValueField) Read(c *Context) {
	c.Putln("// reading valueparam: type: %s, mask name: %s, list name: %s",
		f.MaskType.SrcName(), f.MaskName, f.ListName)
}

// Switch field
func (f *SwitchField) Define(c *Context) {
	c.Putln("// switch field: %s (%s)", f.Name, f.Expr)
}

func (f *SwitchField) Read(c *Context) {
	c.Putln("// reading switch field: %s (%s)", f.Name, f.Expr)
}
