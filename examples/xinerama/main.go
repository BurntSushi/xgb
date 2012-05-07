// Example xinerama shows how to query the geometry of all active heads.
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

	// Initialize the Xinerama extension.
	// The appropriate 'Init' function must be run for *every*
	// extension before any of its requests can be used.
	err = X.XineramaInit()
	if err != nil {
		log.Fatal(err)
	}

	// Issue a request to get the screen information.
	reply, err := X.XineramaQueryScreens().Reply()
	if err != nil {
		log.Fatal(err)
	}

	// reply.Number is the number of active heads, while reply.ScreenInfo
	// is a slice of XineramaScreenInfo containing the rectangle geometry
	// of each head.
	fmt.Printf("Number of heads: %d\n", reply.Number)
	for i, screen := range reply.ScreenInfo {
		fmt.Printf("%d :: X: %d, Y: %d, Width: %d, Height: %d\n",
			i, screen.XOrg, screen.YOrg, screen.Width, screen.Height)
	}
}
