package main

import (
	"os"

	"github.com/ngld/knossos/packages/updater/ui"
)

func main() {
	go ui.InitIntroWindow()

	err := ui.RunApp("Knossos Updater", 900, 500)
	if err != nil {
		panic(err)
	}

	os.Exit(0)
}
