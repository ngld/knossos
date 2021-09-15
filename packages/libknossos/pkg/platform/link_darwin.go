package platform

import "os"

func OpenLink(link string) error {
	_, err := os.StartProcess("open", []string{link}, nil)
	return err
}
