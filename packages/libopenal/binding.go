package libopenal

// #include <stdlib.h>
// #include <string.h>
// #include <AL/alc.h>
// #cgo windows CFLAGS: -I${SRCDIR}/../../third_party/openal/include
// #cgo darwin  CFLAGS: -I${SRCDIR}/../../third_party/openal/include
// #cgo windows LDFLAGS: -L${SRCDIR}/../../third_party/openal/bin/Win64 -lsoft_oal
// #cgo linux   LDFLAGS: -lopenal
// #cgo darwin  LDFLAGS: -framework OpenAL
import "C"

import (
	"context"
	"unsafe"
)

type OpenALDeviceInfo struct {
	DefaultDevice  string
	DefaultCapture string
	Devices        []string
	Captures       []string
}

func splitZeroString(input *C.char) []string {
	result := make([]string, 0)
	if input == nil {
		return result
	}

	charLen := unsafe.Sizeof(*input)

	for {
		itemLen := C.strlen(input)
		if itemLen == 0 {
			break
		}

		result = append(result, C.GoString(input))
		input = (*C.char)(unsafe.Add(unsafe.Pointer(input), uintptr(itemLen+1)*charLen))
	}

	return result
}

func GetDeviceInfo(ctx context.Context) (OpenALDeviceInfo, error) {
	var info OpenALDeviceInfo

	info.Devices = splitZeroString(C.alcGetString(nil, C.ALC_ALL_DEVICES_SPECIFIER))
	info.DefaultDevice = C.GoString(C.alcGetString(nil, C.ALC_DEFAULT_DEVICE_SPECIFIER))
	info.Captures = splitZeroString(C.alcGetString(nil, C.ALC_CAPTURE_DEVICE_SPECIFIER))
	info.DefaultCapture = C.GoString(C.alcGetString(nil, C.ALC_CAPTURE_DEFAULT_DEVICE_SPECIFIER))

	return info, nil
}
