package platform

import (
	"github.com/rotisserie/eris"
	"golang.org/x/sys/windows"
)

func OpenLink(link string) error {
	verb, err := windows.UTF16PtrFromString("open")
	if err != nil {
		return eris.Wrap(err, "failed to convert verb string")
	}

	linkPtr, err := windows.UTF16PtrFromString(link)
	if err != nil {
		return eris.Wrap(err, "failed to convert link string")
	}

	err = windows.ShellExecute(windows.InvalidHandle, verb, linkPtr, nil, nil, windows.SW_NORMAL)
	if err != nil {
		return eris.Wrap(err, "failed to open link")
	}

	return nil
}
