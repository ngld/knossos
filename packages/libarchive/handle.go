package libarchive

// #cgo CFLAGS: -I${SRCDIR}/../../third_party/libarchive
// #cgo LDFLAGS: -L${SRCDIR}/../../build/libarchive/libarchive
//
// #include <stdlib.h>
// #include <libarchive/archive.h>
// #include <libarchive/archive_entry.h>
import "C"

import (
	"io"
	"os"
	"runtime"
	"unsafe"

	"github.com/rotisserie/eris"
)

type Archive struct {
	handle     *C.struct_archive
	buffer     unsafe.Pointer
	bufferSize int
	Filename   string
	Entry      Header
}

type Header struct {
	Pathname string
	Mode     os.FileMode
	Size     int64
}

func CompiledVersion() int {
	return int(C.ARCHIVE_VERSION_NUMBER)
}

func Version() int {
	return int(C.archive_version_number())
}

func OpenArchive(filename string) (*Archive, error) {
	a := new(Archive)
	a.handle = C.archive_read_new()
	a.Filename = filename

	if a.handle == nil {
		return nil, ErrAlloc
	}

	// Safety net
	runtime.SetFinalizer(a, func(obj *Archive) {
		obj.Close()
	})

	C.archive_read_support_filter_bzip2(a.handle)
	C.archive_read_support_filter_gzip(a.handle)
	C.archive_read_support_filter_lz4(a.handle)
	C.archive_read_support_filter_lzma(a.handle)
	C.archive_read_support_filter_lzop(a.handle)
	C.archive_read_support_filter_xz(a.handle)
	C.archive_read_support_filter_zstd(a.handle)

	C.archive_read_support_format_7zip(a.handle)
	C.archive_read_support_format_tar(a.handle)
	C.archive_read_support_format_rar(a.handle)
	C.archive_read_support_format_zip(a.handle)

	cfilename := C.CString(a.Filename)
	code := C.archive_read_open_filename(a.handle, cfilename, 4096)
	C.free(unsafe.Pointer(cfilename))

	if code != C.ARCHIVE_OK {
		err := a.Error()
		a.Close()
		return nil, err
	}

	return a, nil
}

func (a *Archive) Error() error {
	return a.code2error(C.archive_errno(a.handle))
}

func (a *Archive) code2error(code C.int) error {
	if code == C.ARCHIVE_OK {
		return nil
	}

	if code == C.ARCHIVE_EOF {
		return io.EOF
	}

	msg := C.GoString(C.archive_error_string(a.handle))
	return eris.Errorf("%d: %s", code, msg)
}

func (a *Archive) Next() error {
	var entry *C.struct_archive_entry
	code := C.archive_read_next_header(a.handle, &entry)
	if code != C.ARCHIVE_OK {
		return a.code2error(code)
	}

	a.Entry.Pathname = C.GoString(C.archive_entry_pathname(entry))
	a.Entry.Mode = os.FileMode(C.archive_entry_mode(entry))
	a.Entry.Size = int64(C.archive_entry_size(entry))
	return nil
}

func (a *Archive) Read(buffer []byte) (int, error) {
	bufferSize := len(buffer)

	if a.bufferSize < bufferSize {
		C.free(a.buffer)
		a.buffer = nil
	}

	if a.buffer == nil {
		a.buffer = C.malloc(C.size_t(bufferSize))
		a.bufferSize = bufferSize
	}

	read := C.archive_read_data(a.handle, a.buffer, C.size_t(bufferSize))
	if read > 0 {
		goBuffer := C.GoBytes(a.buffer, C.int(read))
		copy(buffer, goBuffer)
	} else {
		return 0, io.EOF
	}

	return int(read), nil
}

func (a *Archive) Close() error {
	C.archive_read_free(a.handle)

	if a.buffer != nil {
		C.free(a.buffer)
	}

	runtime.SetFinalizer(a, nil)
	return nil
}
