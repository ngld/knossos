//go:build 386 && linux
// +build 386,linux

package helpers

import "os"

var cachedX64Support *bool = nil

func SupportsX64() bool {
	if cachedX64Support == nil {
		cachedX64Support = new(bool)

		// 64bit Linux distros usually have a /lib64 directory (even though it's often a symlink to /usr/lib64)
		_, err := os.Stat("/lib64")
		*cachedX64Support = err == nil
	}

	return *cachedX64Support
}
