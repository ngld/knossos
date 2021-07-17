// +build arm64
package helpers

// macOS supports x64 through Rosetta2
func SupportsX64() bool { return runtime.GOOS == "darwin" }
