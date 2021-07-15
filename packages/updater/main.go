package main

import (
	"fmt"
	"os"

	"github.com/ngld/knossos/packages/updater/platform"
	"github.com/ngld/knossos/packages/updater/ui"
)

func main() {
	go ui.InitIntroWindow()

	err := ui.RunApp("Knossos Updater", 900, 500)
	if err != nil {
		platform.ShowError(fmt.Sprintf("Encountered fatal error:\n%s", err))
		panic(err)
	}

	os.Exit(0)
}
