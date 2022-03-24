//go:build !cgo
// +build !cgo

// This file contains a bunch of stubs so gopls can still function with CGO_ENABLED=0.
// The stubs always return an error since we don't support building without CGo.

package libarchive

import (
	"os"
	"unsafe"

	"github.com/rotisserie/eris"
)

type Archive struct {
	handle     unsafe.Pointer
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
	return 0
}

func Version() int {
	return 0
}

func OpenArchive(filename string) (*Archive, error) {
	return nil, eris.New("stub")
}

func (a *Archive) Error() error {
	return eris.New("stub")
}

func (a *Archive) Next() error {
	return eris.New("stub")
}

func (a *Archive) Read(buffer []byte) (int, error) {
	return 0, eris.New("stub")
}

func (a *Archive) Close() error {
	return eris.New("stub")
}

type ArchiveWriter struct {
	handle     unsafe.Pointer
	entry      unsafe.Pointer
	buffer     unsafe.Pointer
	Filename   string
	bufferSize int
}

func CreateArchive(filename string) (*ArchiveWriter, error) {
	return nil, eris.New("stub")
}

func (w *ArchiveWriter) CreateFile(filename string, mode uint32, size int64) error {
	return eris.New("stub")
}

func (w *ArchiveWriter) Write(buffer []byte) (int, error) {
	return 0, eris.New("stub")
}

func (w *ArchiveWriter) Error() error {
	return eris.New("stub")
}

func (w *ArchiveWriter) Close() error {
	return eris.New("stub")
}
