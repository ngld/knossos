package platform

import (
	"os"

	"github.com/rotisserie/eris"
)

func OpenLink(link string) error {
	_, err := os.StartProcess("open", []string{link}, nil)
	if err != nil {
		eris.Wrap(err, "failed to launch open")
	}

	return nil
}
