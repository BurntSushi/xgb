package main

import (
	"fmt"
	"log"

	"github.com/BurntSushi/xgb"
)

func main() {
	X, _ := xgb.NewConn()

	err := X.RegisterExtension("xinerama")
	if err != nil {
		log.Fatal(err)
	}

	reply, err := X.XineramaQueryScreens().Reply()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Xinerama number: %d\n", reply.Number)
	for i, screen := range reply.ScreenInfo {
		fmt.Printf("%d :: X: %d, Y: %d, Width: %d, Height: %d\n",
			i, screen.XOrg, screen.YOrg, screen.Width, screen.Height)
	}
}

