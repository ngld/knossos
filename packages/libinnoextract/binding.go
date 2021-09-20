package libinnoextract

// #include <stdlib.h>
// #include <stdint.h>
// #include "binding.h"
// #cgo linux  LDFLAGS: -ldl
// #cgo darwin LDFLAGS: -ldl
import "C"

import (
	"sync"
	"unsafe"

	"github.com/rotisserie/eris"
)

//go:generate stringer -type=LogLevel
type LogLevel int

const (
	LogDebug LogLevel = iota + 1
	LogInfo
	LogWarning
	LogError
)

type (
	ProgressCb func(float32, string)
	LogCb      func(LogLevel, string)
)

var (
	lock          sync.Mutex
	currentProgCb ProgressCb
	currentLogCb  LogCb
	loaded        bool = false
)

//export libinnoextract_progress_cb
func libinnoextract_progress_cb(message *C.char, progress C.float) {
	currentProgCb(float32(progress), C.GoString(message))
}

//export libinnoextract_log_cb
func libinnoextract_log_cb(level C.uint8_t, message *C.char) {
	currentLogCb(LogLevel(level), C.GoString(message))
}

func LoadLibrary(libPath string) error {
	if loaded {
		return nil
	}

	var errorPtr *C.char

	cLibPath := C.CString(libPath)
	defer C.free(unsafe.Pointer(cLibPath))

	success := C.load_libinnoextract(cLibPath, &errorPtr)
	if !success {
		return eris.New(C.GoString(errorPtr))
	}

	loaded = true
	return nil
}

func ExtractInstaller(installer, destination string, progCb ProgressCb, logCb LogCb) error {
	// innoextract isn't thread-safe so we'll have to make sure we only ever call it once
	lock.Lock()
	defer lock.Unlock()

	if !loaded {
		return eris.New("innoextract library not loaded")
	}

	currentProgCb = progCb
	currentLogCb = logCb

	cInstaller := C.CString(installer)
	cDest := C.CString(destination)

	defer C.free(unsafe.Pointer(cInstaller))
	defer C.free(unsafe.Pointer(cDest))

	if C.extract_inno_wrapper(cInstaller, cDest) {
		return nil
	}

	return eris.Errorf("innoextract failed, check log for details (file was %s)", installer)
}
