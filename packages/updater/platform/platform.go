package platform

// #cgo darwin LDFLAGS: -framework CoreFoundation -framework Cocoa
// #cgo linux pkg-config: gtk+-3.0 x11
// #cgo windows LDFLAGS: -lole32 -lshell32
// #include <stdlib.h>
// #include "platform.h"
import "C"

import (
	"unsafe"

	"github.com/rotisserie/eris"
)

func init() {
	C.PlatformInit()
}

func ShowError(msg string) {
	cmsg := C.CString(msg)
	C.ShowError(cmsg)
	C.free(unsafe.Pointer(cmsg))
}

func OpenFolder(title, defaultPath string) (string, error) {
	ctitle := C.CString(title)
	cdefaultPath := C.CString(defaultPath)

	defer C.free(unsafe.Pointer(ctitle))
	defer C.free(unsafe.Pointer(cdefaultPath))

	result := C.OpenFolderDialog(ctitle, cdefaultPath)

	if result.code == 0 {
		resultPath := C.GoString(result.string)
		C.free(unsafe.Pointer(result.string))

		return resultPath, nil
	}
	return "", eris.Errorf("Failed with code %d", result.code)
}
