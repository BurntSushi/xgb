// Example randr uses the randr protocol to get information about the active
// heads. It also listens for events that are sent when the head configuration
// changes. Since it listens to events, you'll have to manually kill this
// process when you're done (i.e., ctrl+c.)
//
// While this program is running, if you use 'xrandr' to reconfigure your
// heads, you should see event information dumped to standard out.
//
// For more information, please see the RandR protocol spec:
// http://www.x.org/releases/X11R7.6/doc/randrproto/randrproto.txt
package main

import (
	"fmt"
	"log"

	"github.com/BurntSushi/xgb"
)

func main() {
	X, _ := xgb.NewConn()

	// Every extension must be initialized before it can be used.
	err := X.RandrInit()
	if err != nil {
		log.Fatal(err)
	}

	// Gets the current screen resources. Screen resources contains a list
	// of names, crtcs, outputs and modes, among other things.
	resources, err := X.RandrGetScreenResources(X.DefaultScreen().Root).Reply()
	if err != nil {
		log.Fatal(err)
	}

	// Iterate through all of the outputs and show some of their info.
	for _, output := range resources.Outputs {
		info, err := X.RandrGetOutputInfo(output, 0).Reply()
		if err != nil {
			log.Fatal(err)
		}

		bestMode := info.Modes[0]
		for _, mode := range resources.Modes {
			if mode.Id == uint32(bestMode) {
				fmt.Printf("Width: %d, Height: %d\n", mode.Width, mode.Height)
			}
		}
	}

	fmt.Println("\n")

	// Iterate through all of the crtcs and show some of their info.
	for _, crtc := range resources.Crtcs {
		info, err := X.RandrGetCrtcInfo(crtc, 0).Reply()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("X: %d, Y: %d, Width: %d, Height: %d\n",
			info.X, info.Y, info.Width, info.Height)
	}

	// Tell RandR to send us events. (I think these are all of them, as of 1.3.)
	err = X.RandrSelectInputChecked(X.DefaultScreen().Root,
		xgb.RandrNotifyMaskScreenChange|
			xgb.RandrNotifyMaskCrtcChange|
			xgb.RandrNotifyMaskOutputChange|
			xgb.RandrNotifyMaskOutputProperty).Check()
	if err != nil {
		log.Fatal(err)
	}

	// Listen to events and just dump them to standard out.
	// A more involved approach will have to read the 'U' field of
	// RandrNotifyEvent, which is a union (really a struct) of type
	// RanrNotifyDataUnion.
	for {
		ev, err := X.WaitForEvent()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(ev)
	}
}
