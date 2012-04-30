package main

type Field interface {
	Initialize(p *Protocol)
	SrcName() string
	XmlName() string
	Size() Size

	Define(c *Context)
	Read(c *Context)
}

func (pad *PadField) Initialize(p *Protocol) {}

type PadField struct {
	Bytes uint
}

func (p *PadField) SrcName() string {
	panic("illegal to take source name of a pad field")
}

func (p *PadField) XmlName() string {
	panic("illegal to take XML name of a pad field")
}

func (p *PadField) Size() Size {
	return newFixedSize(p.Bytes)
}

type SingleField struct {
	srcName string
	xmlName string
	Type    Type
}

func (f *SingleField) Initialize(p *Protocol) {
	f.srcName = SrcName(f.XmlName())
	f.Type = f.Type.(*Translation).RealType(p)
}

func (f *SingleField) SrcName() string {
	return f.srcName
}

func (f *SingleField) XmlName() string {
	return f.xmlName
}

func (f *SingleField) Size() Size {
	return f.Type.Size()
}

type ListField struct {
	srcName    string
	xmlName    string
	Type       Type
	LengthExpr Expression
}

func (f *ListField) SrcName() string {
	return f.srcName
}

func (f *ListField) XmlName() string {
	return f.xmlName
}

func (f *ListField) Size() Size {
	return newExpressionSize(f.LengthExpr).Multiply(f.Type.Size())
}

func (f *ListField) Initialize(p *Protocol) {
	f.srcName = SrcName(f.XmlName())
	f.Type = f.Type.(*Translation).RealType(p)
	if f.LengthExpr != nil {
		f.LengthExpr.Initialize(p)
	}
}

type LocalField struct {
	*SingleField
}

type ExprField struct {
	srcName string
	xmlName string
	Type    Type
	Expr    Expression
}

func (f *ExprField) SrcName() string {
	return f.srcName
}

func (f *ExprField) XmlName() string {
	return f.xmlName
}

func (f *ExprField) Size() Size {
	return f.Type.Size()
}

func (f *ExprField) Initialize(p *Protocol) {
	f.srcName = SrcName(f.XmlName())
	f.Type = f.Type.(*Translation).RealType(p)
	f.Expr.Initialize(p)
}

type ValueField struct {
	MaskType Type
	MaskName string
	ListName string
}

func (f *ValueField) SrcName() string {
	panic("it is illegal to call SrcName on a ValueField field")
}

func (f *ValueField) XmlName() string {
	panic("it is illegal to call XmlName on a ValueField field")
}

func (f *ValueField) Size() Size {
	return f.MaskType.Size()
}

func (f *ValueField) Initialize(p *Protocol) {
	f.MaskType = f.MaskType.(*Translation).RealType(p)
	f.MaskName = SrcName(f.MaskName)
	f.ListName = SrcName(f.ListName)
}

type SwitchField struct {
	Name     string
	Expr     Expression
	Bitcases []*Bitcase
}

func (f *SwitchField) SrcName() string {
	panic("it is illegal to call SrcName on a SwitchField field")
}

func (f *SwitchField) XmlName() string {
	panic("it is illegal to call XmlName on a SwitchField field")
}

// XXX: This is a bit tricky. The size has to be represented as a non-concrete
// expression that finds *which* bitcase fields are included, and sums the
// sizes of those fields.
func (f *SwitchField) Size() Size {
	return newFixedSize(0)
}

func (f *SwitchField) Initialize(p *Protocol) {
	f.Name = SrcName(f.Name)
	f.Expr.Initialize(p)
	for _, bitcase := range f.Bitcases {
		bitcase.Expr.Initialize(p)
		for _, field := range bitcase.Fields {
			field.Initialize(p)
		}
	}
}

type Bitcase struct {
	Fields []Field
	Expr   Expression
}
