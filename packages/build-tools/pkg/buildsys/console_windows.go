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
		// not running in a valid console; ignore
		return nil
	}

	if mode&windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING == 0 {
		mode |= windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING
		return windows.SetConsoleMode(outHdl, mode)
	}
	return nil
}
