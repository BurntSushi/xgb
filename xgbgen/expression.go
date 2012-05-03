package main

import (
	"fmt"
	"log"
)

type Expression interface {
	Concrete() bool
	Eval() uint
	Reduce(prefix string) string
	String() string
	Initialize(p *Protocol)
}

// Function is a custom expression not found in the XML. It's simply used
// to apply a function named in 'Name' to the Expr expression.
type Function struct {
	Name string
	Expr Expression
}

func (e *Function) Concrete() bool {
	return false
}

func (e *Function) Eval() uint {
	log.Fatalf("Cannot evaluate a 'Function'. It is not concrete.")
	panic("unreachable")
}

func (e *Function) Reduce(prefix string) string {
	return fmt.Sprintf("%s(%s)", e.Name, e.Expr.Reduce(prefix))
}

func (e *Function) String() string {
	return e.Reduce("")
}

func (e *Function) Initialize(p *Protocol) {
	e.Expr.Initialize(p)
}

type BinaryOp struct {
	Op    string
	Expr1 Expression
	Expr2 Expression
}

func newBinaryOp(op string, expr1, expr2 Expression) Expression {
	switch {
	case expr1 != nil && expr2 != nil:
		return &BinaryOp{
			Op:    op,
			Expr1: expr1,
			Expr2: expr2,
		}
	case expr1 != nil && expr2 == nil:
		return expr1
	case expr1 == nil && expr2 != nil:
		return expr2
	case expr1 == nil && expr2 == nil:
		return nil
	}
	panic("unreachable")
}

func (e *BinaryOp) Concrete() bool {
	return e.Expr1.Concrete() && e.Expr2.Concrete()
}

func (e *BinaryOp) Eval() uint {
	switch e.Op {
	case "+":
		return e.Expr1.Eval() + e.Expr2.Eval()
	case "-":
		return e.Expr1.Eval() - e.Expr2.Eval()
	case "*":
		return e.Expr1.Eval() * e.Expr2.Eval()
	case "/":
		return e.Expr1.Eval() / e.Expr2.Eval()
	case "&amp;":
		return e.Expr1.Eval() & e.Expr2.Eval()
	case "&lt;&lt;":
		return e.Expr1.Eval() << e.Expr2.Eval()
	}

	log.Fatalf("Invalid binary operator '%s' for expression.", e.Op)
	panic("unreachable")
}

func (e *BinaryOp) Reduce(prefix string) string {
	if e.Concrete() {
		return fmt.Sprintf("%d", e.Eval())
	}

	// An incredibly dirty hack to make sure any time we perform an operation
	// on a field, we're dealing with ints...
	expr1, expr2 := e.Expr1, e.Expr2
	switch expr1.(type) {
	case *FieldRef:
		expr1 = &Function{
			Name: "int",
			Expr: expr1,
		}
	}
	switch expr2.(type) {
	case *FieldRef:
		expr2 = &Function{
			Name: "int",
			Expr: expr2,
		}
	}
	return fmt.Sprintf("(%s %s %s)",
		expr1.Reduce(prefix), e.Op, expr2.Reduce(prefix))
}

func (e *BinaryOp) String() string {
	return e.Reduce("")
}

func (e *BinaryOp) Initialize(p *Protocol) {
	e.Expr1.Initialize(p)
	e.Expr2.Initialize(p)
}

type UnaryOp struct {
	Op   string
	Expr Expression
}

func (e *UnaryOp) Concrete() bool {
	return e.Expr.Concrete()
}

func (e *UnaryOp) Eval() uint {
	switch e.Op {
	case "~":
		return ^e.Expr.Eval()
	}

	log.Fatalf("Invalid unary operator '%s' for expression.", e.Op)
	panic("unreachable")
}

func (e *UnaryOp) Reduce(prefix string) string {
	if e.Concrete() {
		return fmt.Sprintf("%d", e.Eval())
	}
	return fmt.Sprintf("(%s (%s))", e.Op, e.Expr.Reduce(prefix))
}

func (e *UnaryOp) String() string {
	return e.Reduce("")
}

func (e *UnaryOp) Initialize(p *Protocol) {
	e.Expr.Initialize(p)
}

type PopCount struct {
	Expr Expression
}

func (e *PopCount) Concrete() bool {
	return e.Expr.Concrete()
}

func (e *PopCount) Eval() uint {
	return popCount(e.Expr.Eval())
}

func (e *PopCount) Reduce(prefix string) string {
	if e.Concrete() {
		return fmt.Sprintf("%d", e.Eval())
	}
	return fmt.Sprintf("popCount(%s)", e.Expr.Reduce(prefix))
}

func (e *PopCount) String() string {
	return e.Reduce("")
}

func (e *PopCount) Initialize(p *Protocol) {
	e.Expr.Initialize(p)
}

type Value struct {
	v uint
}

func (e *Value) Concrete() bool {
	return true
}

func (e *Value) Eval() uint {
	return e.v
}

func (e *Value) Reduce(prefix string) string {
	return fmt.Sprintf("%d", e.v)
}

func (e *Value) String() string {
	return e.Reduce("")
}

func (e *Value) Initialize(p *Protocol) {}

type Bit struct {
	b uint
}

func (e *Bit) Concrete() bool {
	return true
}

func (e *Bit) Eval() uint {
	return 1 << e.b
}

func (e *Bit) Reduce(prefix string) string {
	return fmt.Sprintf("%d", e.Eval())
}

func (e *Bit) String() string {
	return e.Reduce("")
}

func (e *Bit) Initialize(p *Protocol) {}

type FieldRef struct {
	Name string
}

func (e *FieldRef) Concrete() bool {
	return false
}

func (e *FieldRef) Eval() uint {
	log.Fatalf("Cannot evaluate a 'FieldRef'. It is not concrete.")
	panic("unreachable")
}

func (e *FieldRef) Reduce(prefix string) string {
	val := e.Name
	if len(prefix) > 0 {
		val = fmt.Sprintf("%s%s", prefix, val)
	}
	return val
}

func (e *FieldRef) String() string {
	return e.Reduce("")
}

func (e *FieldRef) Initialize(p *Protocol) {
	e.Name = SrcName(e.Name)
}

type EnumRef struct {
	EnumKind Type
	EnumItem string
}

func (e *EnumRef) Concrete() bool {
	return false
}

func (e *EnumRef) Eval() uint {
	log.Fatalf("Cannot evaluate an 'EnumRef'. It is not concrete.")
	panic("unreachable")
}

func (e *EnumRef) Reduce(prefix string) string {
	return fmt.Sprintf("%s%s", e.EnumKind, e.EnumItem)
}

func (e *EnumRef) String() string {
	return e.Reduce("")
}

func (e *EnumRef) Initialize(p *Protocol) {
	e.EnumKind = e.EnumKind.(*Translation).RealType(p)
	e.EnumItem = SrcName(e.EnumItem)
}

type SumOf struct {
	Name string
}

func (e *SumOf) Concrete() bool {
	return false
}

func (e *SumOf) Eval() uint {
	log.Fatalf("Cannot evaluate a 'SumOf'. It is not concrete.")
	panic("unreachable")
}

func (e *SumOf) Reduce(prefix string) string {
	if len(prefix) > 0 {
		return fmt.Sprintf("sum(%s%s)", prefix, e.Name)
	}
	return fmt.Sprintf("sum(%s)", e.Name)
}

func (e *SumOf) String() string {
	return e.Reduce("")
}

func (e *SumOf) Initialize(p *Protocol) {
	e.Name = SrcName(e.Name)
}
