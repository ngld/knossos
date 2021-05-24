package buildsys

import (
	"os"

	"golang.org/x/sys/windows"
)

func resetConsole() error {
	var mode uint32

	outHdl := windows.Handle(os.Stdout.Fd())
	err := windows.GetConsoleMode(outHdl, &mode)
	if err != nil {
		return err
	}

	if mode&windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING == 0 {
		mode |= windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING
		return windows.SetConsoleMode(outHdl, mode)
	}
	return nil
}
