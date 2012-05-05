package main

import (
	// "fmt" 
	"log"

	"github.com/BurntSushi/xgb"
)

func init() {
	log.SetFlags(0)
}

func main() {
	X, err := xgb.NewConn()
	if err != nil {
		log.Fatal(err)
	}

	aname := "_NET_ACTIVE_WINDOW"
	atom, err := X.InternAtom(true, uint16(len(aname)), aname).Reply()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("%d", atom.Atom)
}

