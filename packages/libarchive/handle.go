package libarchive

// #cgo CFLAGS: -I${SRCDIR}/../../third_party/libarchive
// #cgo LDFLAGS: -L${SRCDIR}/../../build/libarchive/libarchive
//
// #include <stdlib.h>
// #include <libarchive/archive.h>
// #include <libarchive/archive_entry.h>
//
// inline void archive_entry_set_mode_helper(struct archive_entry *entry, uint32_t mode) {
//   archive_entry_set_mode(entry, (__LA_MODE_T)mode);
// }
import "C"

import (
	"io"
	"os"
	"runtime"
	"unsafe"

	"github.com/rotisserie/eris"
)

var knossosStr = C.CString("knossos")

type Archive struct {
	handle     *C.struct_archive
	buffer     unsafe.Pointer
	Filename   string
	Entry      Header
	bufferSize int
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

	// Safety net
	runtime.SetFinalizer(a, func(obj *Archive) {
		obj.Close()
	})

	a.handle = C.archive_read_new()
	if a.handle == nil {
		return nil, ErrAlloc
	}

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

	a.Filename = filename

	cfilename := C.CString(a.Filename)
	code := C.archive_read_open_filename(a.handle, cfilename, 4096)
	C.free(unsafe.Pointer(cfilename))

	if code != C.ARCHIVE_OK {
		err := a.code2error(code)
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

	if a.bufferSize < bufferSize && a.buffer != nil {
		C.free(a.buffer)
		a.buffer = nil
	}

	if a.buffer == nil {
		a.buffer = C.malloc(C.size_t(bufferSize))
		a.bufferSize = bufferSize
	}

	read := C.archive_read_data(a.handle, a.buffer, C.size_t(bufferSize))
	if read > 0 {
		goBuffer := (*[1 << 30]byte)(a.buffer)[:bufferSize]
		/* safer (and slower) version
		goBuffer := C.GoBytes(a.buffer, C.int(read)) */
		copy(buffer, goBuffer)
	} else {
		return 0, io.EOF
	}

	return int(read), nil
}

func (a *Archive) Close() error {
	if a.handle != nil {
		C.archive_read_free(a.handle)
		a.handle = nil
	}

	if a.buffer != nil {
		C.free(a.buffer)
		a.buffer = nil
	}

	runtime.SetFinalizer(a, nil)
	return nil
}

type ArchiveWriter struct {
	handle     *C.struct_archive
	entry      *C.struct_archive_entry
	buffer     unsafe.Pointer
	Filename   string
	bufferSize int
}

func CreateArchive(filename string) (*ArchiveWriter, error) {
	w := new(ArchiveWriter)
	// Safety net
	runtime.SetFinalizer(w, func(obj *ArchiveWriter) {
		obj.Close()
	})

	w.handle = C.archive_write_new()
	if w.handle == nil {
		return nil, ErrAlloc
	}

	cfilename := C.CString(filename)
	code := C.archive_write_set_format_filter_by_ext(w.handle, cfilename)
	if err := w.code2error(code); err != nil {
		C.free(unsafe.Pointer(cfilename))
		w.Close()
		return nil, err
	}

	code = C.archive_write_open_filename(w.handle, cfilename)
	if err := w.code2error(code); err != nil {
		C.free(unsafe.Pointer(cfilename))
		w.Close()
		return nil, err
	}
	C.free(unsafe.Pointer(cfilename))

	w.entry = C.archive_entry_new2(w.handle)
	if w.entry == nil {
		w.Close()
		return nil, ErrAlloc
	}

	return w, nil
}

func (w *ArchiveWriter) CreateFile(filename string, mode uint32, size int64) error {
	C.archive_entry_clear(w.entry)

	C.archive_entry_set_uname(w.entry, knossosStr)
	C.archive_entry_set_gname(w.entry, knossosStr)
	C.archive_entry_set_mode_helper(w.entry, C.uint32_t(mode))
	C.archive_entry_set_size(w.entry, C.int64_t(size))

	cfilename := C.CString(filename)
	C.archive_entry_set_pathname_utf8(w.entry, cfilename)
	C.free(unsafe.Pointer(cfilename))

	code := C.archive_write_header(w.handle, w.entry)
	if code != C.ARCHIVE_OK {
		return w.code2error(code)
	}

	return nil
}

func (w *ArchiveWriter) Write(buffer []byte) (int, error) {
	bufferSize := len(buffer)

	if w.bufferSize < bufferSize && w.buffer != nil {
		C.free(w.buffer)
		w.buffer = nil
	}

	if w.buffer == nil {
		w.buffer = C.malloc(C.size_t(bufferSize))
		w.bufferSize = bufferSize
	}

	bufferPtr := (*[1 << 30]byte)(w.buffer)[:bufferSize]
	copy(bufferPtr, buffer)

	written := C.archive_write_data(w.handle, w.buffer, C.size_t(bufferSize))
	if int(written) != bufferSize {
		return int(written), w.Error()
	}

	return int(written), nil
}

func (w *ArchiveWriter) Error() error {
	return w.code2error(C.archive_errno(w.handle))
}

func (w *ArchiveWriter) code2error(code C.int) error {
	if code == C.ARCHIVE_OK {
		return nil
	}

	if code == C.ARCHIVE_EOF {
		return io.EOF
	}

	msg := C.GoString(C.archive_error_string(w.handle))
	return eris.Errorf("%d: %s", code, msg)
}

func (w *ArchiveWriter) Close() error {
	if w.entry != nil {
		C.archive_entry_free(w.entry)
		w.entry = nil
	}

	if w.buffer != nil {
		C.free(w.buffer)
		w.buffer = nil
	}

	if w.handle != nil {
		code := C.archive_write_free(w.handle)
		if code != C.ARCHIVE_OK {
			runtime.SetFinalizer(w, nil)
			return w.code2error(code)
		}
	}

	runtime.SetFinalizer(w, nil)
	return nil
}
