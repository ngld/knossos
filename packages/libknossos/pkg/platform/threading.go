//go:build !darwin

package platform

func RunOnMain(callback func()) {
	panic("RunOnMain() is not implemented on this platform")
}
