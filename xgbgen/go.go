package main

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

// Field definitions, reads and writes.

// Pad fields
func (f *PadField) Define(c *Context) {
	c.Putln("// padding: %d bytes", f.Bytes)
}

func (f *PadField) Read(c *Context) {
	c.Putln("b += %s // padding", f.Size())
}

func (f *PadField) Write(c *Context) {
	c.Putln("b += %s // padding", f.Size())
}

// Local fields
func (f *LocalField) Define(c *Context) {
	c.Putln("// local field: %s %s", f.SrcName(), f.Type.SrcName())
	panic("todo")
}

func (f *LocalField) Read(c *Context) {
	c.Putln("// reading local field: %s (%s) :: %s",
		f.SrcName(), f.Size(), f.Type.SrcName())
	panic("todo")
}

func (f *LocalField) Write(c *Context) {
	c.Putln("// writing local field: %s (%s) :: %s",
		f.SrcName(), f.Size(), f.Type.SrcName())
	panic("todo")
}

// Expr fields
func (f *ExprField) Define(c *Context) {
	c.Putln("// expression field: %s %s (%s)",
		f.SrcName(), f.Type.SrcName(), f.Expr)
	panic("todo")
}

func (f *ExprField) Read(c *Context) {
	c.Putln("// reading expression field: %s (%s) (%s) :: %s",
		f.SrcName(), f.Size(), f.Expr, f.Type.SrcName())
	panic("todo")
}

func (f *ExprField) Write(c *Context) {
	c.Putln("// writing expression field: %s (%s) (%s) :: %s",
		f.SrcName(), f.Size(), f.Expr, f.Type.SrcName())
	panic("todo")
}

// Value field
func (f *ValueField) Define(c *Context) {
	c.Putln("// valueparam field: type: %s, mask name: %s, list name: %s",
		f.MaskType.SrcName(), f.MaskName, f.ListName)
	panic("todo")
}

func (f *ValueField) Read(c *Context) {
	c.Putln("// reading valueparam: type: %s, mask name: %s, list name: %s",
		f.MaskType.SrcName(), f.MaskName, f.ListName)
	panic("todo")
}

func (f *ValueField) Write(c *Context) {
	c.Putln("// writing valueparam: type: %s, mask name: %s, list name: %s",
		f.MaskType.SrcName(), f.MaskName, f.ListName)
	panic("todo")
}

// Switch field
func (f *SwitchField) Define(c *Context) {
	c.Putln("// switch field: %s (%s)", f.Name, f.Expr)
	panic("todo")
}

func (f *SwitchField) Read(c *Context) {
	c.Putln("// reading switch field: %s (%s)", f.Name, f.Expr)
	panic("todo")
}

func (f *SwitchField) Write(c *Context) {
	c.Putln("// writing switch field: %s (%s)", f.Name, f.Expr)
	panic("todo")
}
