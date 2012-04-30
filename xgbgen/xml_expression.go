package main

import (
	"encoding/xml"
	"fmt"
	"log"
	"strconv"
)

type XMLExpression struct {
	XMLName xml.Name

	Exprs []*XMLExpression `xml:",any"`

	Data string `xml:",chardata"`
	Op string `xml:"op,attr"`
	Ref string `xml:"ref,attr"`
}

func newValueExpression(v uint) *XMLExpression {
	return &XMLExpression{
		XMLName: xml.Name{Local: "value"},
		Data: fmt.Sprintf("%d", v),
	}
}

// String is for debugging. For actual use, please use 'Morph'.
func (e *XMLExpression) String() string {
	switch e.XMLName.Local {
	case "op":
		return fmt.Sprintf("(%s %s %s)", e.Exprs[0], e.Op, e.Exprs[1])
	case "unop":
		return fmt.Sprintf("(%s (%s))", e.Op, e.Exprs[0])
	case "popcount":
		return fmt.Sprintf("popcount(%s)", e.Exprs[0])
	case "fieldref":
		fallthrough
	case "value":
		return fmt.Sprintf("%s", e.Data)
	case "bit":
		return fmt.Sprintf("(1 << %s)", e.Data)
	case "enumref":
		return fmt.Sprintf("%s%s", e.Ref, e.Data)
	case "sumof":
		return fmt.Sprintf("sum(%s)", e.Ref)
	default:
		log.Panicf("Unrecognized expression element: %s", e.XMLName.Local)
	}

	panic("unreachable")
}

// Eval is used to *attempt* to compute a concrete value for a particular
// expression. This is used in the initial setup to instantiate values for
// empty items in enums.
// We can't compute a concrete value for expressions that rely on a context,
// i.e., some field value.
func (e *XMLExpression) Eval() uint {
	switch e.XMLName.Local {
	case "op":
		if len(e.Exprs) != 2 {
			log.Panicf("'op' found %d expressions; expected 2.", len(e.Exprs))
		}
		return e.BinaryOp(e.Exprs[0], e.Exprs[1]).Eval()
	case "unop":
		if len(e.Exprs) != 1 {
			log.Panicf("'unop' found %d expressions; expected 1.", len(e.Exprs))
		}
		return e.UnaryOp(e.Exprs[0]).Eval()
	case "popcount":
		if len(e.Exprs) != 1 {
			log.Panicf("'popcount' found %d expressions; expected 1.",
				len(e.Exprs))
		}
		return popCount(e.Exprs[0].Eval())
	case "value":
		val, err := strconv.Atoi(e.Data)
		if err != nil {
			log.Panicf("Could not convert '%s' in 'value' expression to int.",
				e.Data)
		}
		return uint(val)
	case "bit":
		bit, err := strconv.Atoi(e.Data)
		if err != nil {
			log.Panicf("Could not convert '%s' in 'bit' expression to int.",
				e.Data)
		}
		if bit < 0 || bit > 31 {
			log.Panicf("A 'bit' literal must be in the range [0, 31], but " +
				" is %d", bit)
		}
		return 1 << uint(bit)
	case "fieldref":
		log.Panicf("Cannot compute concrete value of 'fieldref' in " +
			"expression '%s'.", e)
	case "enumref":
		log.Panicf("Cannot compute concrete value of 'enumref' in " +
			"expression '%s'.", e)
	case "sumof":
		log.Panicf("Cannot compute concrete value of 'sumof' in " +
			"expression '%s'.", e)
	}

	log.Panicf("Unrecognized tag '%s' in expression context. Expected one of " +
		"op, fieldref, value, bit, enumref, unop, sumof or popcount.",
		e.XMLName.Local)
	panic("unreachable")
}

func (e *XMLExpression) BinaryOp(oprnd1, oprnd2 *XMLExpression) *XMLExpression {
	if e.XMLName.Local != "op" {
		log.Panicf("Cannot perform binary operation on non-op expression: %s",
			e.XMLName.Local)
	}
	if len(e.Op) == 0 {
		log.Panicf("Cannot perform binary operation without operator for: %s",
			e.XMLName.Local)
	}

	wrap := newValueExpression
	switch e.Op {
	case "+":
		return wrap(oprnd1.Eval() + oprnd2.Eval())
	case "-":
		return wrap(oprnd1.Eval() + oprnd2.Eval())
	case "*":
		return wrap(oprnd1.Eval() * oprnd2.Eval())
	case "/":
		return wrap(oprnd1.Eval() / oprnd2.Eval())
	case "&amp;":
		return wrap(oprnd1.Eval() & oprnd2.Eval())
	case "&lt;&lt;":
		return wrap(oprnd1.Eval() << oprnd2.Eval())
	}

	log.Panicf("Invalid binary operator '%s' for '%s' expression.",
		e.Op, e.XMLName.Local)
	panic("unreachable")
}

func (e *XMLExpression) UnaryOp(oprnd *XMLExpression) *XMLExpression {
	if e.XMLName.Local != "unop" {
		log.Panicf("Cannot perform unary operation on non-unop expression: %s",
			e.XMLName.Local)
	}
	if len(e.Op) == 0 {
		log.Panicf("Cannot perform unary operation without operator for: %s",
			e.XMLName.Local)
	}

	switch e.Op {
	case "~":
		return newValueExpression(^oprnd.Eval())
	}

	log.Panicf("Invalid unary operator '%s' for '%s' expression.",
		e.Op, e.XMLName.Local)
	panic("unreachable")
}
