package main

import (
	"fmt"

	"github.com/BurntSushi/xgb"
)

func main() {
	X, _ := xgb.NewConn()

	for i := 1; i <= 1<<16 + 10; i++ {
		X.NoOperation()
		// fmt.Printf("%d. No-Op\n", i) 
	}

	aname := "_NET_ACTIVE_WINDOW"

	for i := 1; i <= 10; i++ {
		atom, err := X.InternAtom(true, uint16(len(aname)), aname).Reply()
		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Printf("%d. Sequence: %d, Atom: %d\n",
				i, atom.Sequence, atom.Atom)
		}
	}
}

