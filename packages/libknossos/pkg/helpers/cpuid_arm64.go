//go:build arm64
// +build arm64

package helpers

import "runtime"

// macOS supports x64 through Rosetta2
func SupportsX64() bool { return runtime.GOOS == "darwin" }
