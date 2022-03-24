//go:build !windows
// +build !windows

package buildsys

func resetConsole() error {
	return nil
}
