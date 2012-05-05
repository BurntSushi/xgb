package main

import (
	"fmt"
	"log"

	"github.com/BurntSushi/xgb"
)

func main() {
	X, _ := xgb.NewConn()

	err := X.RegisterExtension("randr")
	if err != nil {
		log.Fatal(err)
	}

	resources, err := X.RandrGetScreenResources(X.DefaultScreen().Root).Reply()
	if err != nil {
		log.Fatal(err)
	}

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

	for _, crtc := range resources.Crtcs {
		info, err := X.RandrGetCrtcInfo(crtc, 0).Reply()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("X: %d, Y: %d, Width: %d, Height: %d\n",
			info.X, info.Y, info.Width, info.Height)
	}
}

