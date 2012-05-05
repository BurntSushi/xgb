package main

import (
	"fmt"
	"log"

	"github.com/BurntSushi/xgb"
)

func main() {
	X, err := xgb.NewConn()
	if err != nil {
		log.Fatal(err)
	}

	wid, _ := X.NewId()
	X.CreateWindow(X.DefaultScreen().RootDepth, wid, X.DefaultScreen().Root,
		0, 0, 500, 500, 0,
		xgb.WindowClassInputOutput, X.DefaultScreen().RootVisual,
		0, []uint32{})
	X.ChangeWindowAttributes(wid, xgb.CwEventMask | xgb.CwBackPixel,
		[]uint32{0xffffffff, xgb.EventMaskKeyPress | xgb.EventMaskKeyRelease})

	err = X.MapWindowChecked(wid).Check()
	if err != nil {
		fmt.Printf("Checked Error for mapping window %d: %s\n", wid, err)
	} else {
		fmt.Printf("Map window %d successful!\n", wid)
	}

	err = X.MapWindowChecked(0x1).Check()
	if err != nil {
		fmt.Printf("Checked Error for mapping window 0x1: %s\n", err)
	} else {
		fmt.Printf("Map window 0x1 successful!\n")
	}

	for {
		ev, xerr := X.WaitForEvent()
		if ev == nil && xerr == nil {
			log.Fatal("Both event and error are nil. Exiting...")
		}

		if ev != nil {
			fmt.Printf("Event: %s\n", ev)
		}
		if xerr != nil {
			fmt.Printf("Error: %s\n", xerr)
		}

		if xerr == nil {
			geom, err := X.GetGeometry(0x1).Reply()
			if err != nil {
				fmt.Printf("Geom Error: %#v\n", err)
			} else {
				fmt.Printf("Geometry: %#v\n", geom)
			}
		}
	}
}

