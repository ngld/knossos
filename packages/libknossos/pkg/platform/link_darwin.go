package platform

import (
	"os"

	"github.com/rotisserie/eris"
)

func OpenLink(link string) error {
	proc := exec.Command("open", link)
	proc.Stderr = os.Stderr
	proc.Stdout = os.Stdout
	proc.Stdin = nil
	err := proc.Start()
	if err != nil {
		return eris.Wrap(err, "failed to launch xdg-open")
	}

	return nil
}
