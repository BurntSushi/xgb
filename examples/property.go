package main

import (
	"log"

	"github.com/BurntSushi/xgb"
)

func init() {
	log.SetFlags(0)
}

func get32(buf []byte) uint32 {
	v := uint32(buf[0])
	v |= uint32(buf[1]) << 8
	v |= uint32(buf[2]) << 16
	v |= uint32(buf[3]) << 24
	return v
}

func main() {
	X, err := xgb.NewConn()
	if err != nil {
		log.Fatal(err)
	}

	root := X.DefaultScreen().Root

	aname := "_NET_ACTIVE_WINDOW"
	atom, err := X.InternAtom(true, uint16(len(aname)), aname)
	if err != nil {
		log.Fatal(err)
	}

	reply, err := X.GetProperty(false, root, atom.Atom, xgb.GetPropertyTypeAny,
		0, (1<<32)-1)
	log.Printf("%X", get32(reply.Value))
}

