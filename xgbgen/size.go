package main

type Size struct {
	Expression
}

func newFixedSize(fixed uint) Size {
	return Size{&Value{v: fixed}}
}

func newExpressionSize(variable Expression) Size {
	return Size{variable}
}

func (s1 Size) Add(s2 Size) Size {
	return Size{newBinaryOp("+", s1, s2)}
}

func (s1 Size) Multiply(s2 Size) Size {
	return Size{newBinaryOp("*", s1, s2)}
}

