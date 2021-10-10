package platform

// #include <stdlib.h>
// #include "platform.h"
import "C"

import (
	"fmt"
	"strings"
	"unsafe"

	"github.com/rotisserie/eris"
)

func RunElevated(program string, args ...string) error {
	cProgram := C.CString(program)
	defer C.free(unsafe.Pointer(cProgram))

	for idx, arg := range args {
		if strings.ContainsAny(arg, "\"^") {
			return eris.New("parameters containing \" or ^ are not supported")
		}

		args[idx] = fmt.Sprintf("\"%s\"", arg)
	}

	cArgs := C.CString(strings.Join(args, " "))
	defer C.free(unsafe.Pointer(cArgs))

	result := C.RunElevated(cProgram, cArgs)
	if result == nil {
		return nil
	}

	message := C.GoString(result)
	C.free(unsafe.Pointer(result))

	return eris.New(message)
}
