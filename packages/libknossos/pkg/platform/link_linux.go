package platform

import "os"

func OpenLink(link string) error {
	_, err := os.StartProcess("xdg-open", []string{link}, nil)
	return err
}
