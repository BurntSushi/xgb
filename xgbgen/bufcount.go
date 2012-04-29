package main

/*
	A buffer count is a mechanism by which to keep track of which byte one
	is reading or writing to/from the wire.

	It's an abstraction over the fact that while such a counter is usually
	fixed, it can be made variable based on values at run-time.
*/

type BufCount struct {
	Fixed int
	Exprs []*Expression
}

