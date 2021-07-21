// +build 386,windows
package helpers

import "golang.org/x/sys/windows"

var cachedX64Support *bool = nil

func SupportsX64() bool {
	if cachedX64Support == nil {
		cachedX64Support = new(bool)

		var result bool
		err := windows.IsWow64Process(windows.CurrentProcess(), &result)
		*cachedX64Support = err == nil && result
	}

	return *cachedX64Support
}
