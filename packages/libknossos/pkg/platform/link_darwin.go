package platform

import (
	"os"

	"golang.org/x/sys/windows"
)

func OpenLink(link string) error {
	_, err := os.StartProcess("open", []string{link}, nil)
	return eris.Wrap(err, "failed to launch open")
}
