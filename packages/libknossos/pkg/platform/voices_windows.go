package platform

/*
#include <stdlib.h>

struct voice_list {
  unsigned long count;
  char** names;
};

struct voice_list get_voices();

#cgo LDFLAGS: -lole32 -lsapi
*/
import "C"

import (
	"context"
	"unsafe"
)

func GetVoices(ctx context.Context) ([]string, error) {
	list := C.get_voices()
	result := make([]string, list.count)
	names := (*[1024]*C.char)(unsafe.Pointer(list.names))

	for idx := range result {
		result[idx] = C.GoString(names[idx])
		C.free(unsafe.Pointer(names[idx]))
	}

	C.free(unsafe.Pointer(list.names))

	return result, nil
}
