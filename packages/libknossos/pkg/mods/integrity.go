package mods

import (
	"bytes"
	"context"
	"crypto/sha256"
	"io"
	"os"
	"path"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ngld/knossos/packages/api/common"
	"github.com/ngld/knossos/packages/libknossos/pkg/api"
	"github.com/rotisserie/eris"
)

func VerifyModIntegrity(ctx context.Context, rel *common.Release) error {
	api.Log(ctx, api.LogInfo, "Fetching checksums")
	checksumPacks, err := FetchModChecksums(ctx, map[string]string{rel.Modid: rel.Version})
	if err != nil {
		return eris.Wrap(err, "failed to acquire checksums")
	}

	modFolder, err := GetModFolder(ctx, rel)
	if err != nil {
		return eris.Wrap(err, "failed to build mod folder")
	}

	checksums := checksumPacks[rel.Modid]
	totalBytes := int64(0)
	for _, pkg := range rel.Packages {
		for _, archive := range pkg.Archives {
			for _, file := range checksums.Archives[archive.Label].Files {
				fpath := path.Join(pkg.Folder, archive.Destination, file.Filename)

				if file.Size < 1 {
					api.Log(ctx, api.LogWarn, "Missing file size for %s", fpath)

					info, err := os.Stat(filepath.Join(modFolder, fpath))
					if err != nil {
						api.Log(ctx, api.LogWarn, "Could not retrieve size of %s; the progress bar will be incorrect", fpath)
					} else {
						file.Size = uint32(info.Size())
					}
				}

				totalBytes += int64(file.Size)
			}
		}
	}

	api.Log(ctx, api.LogInfo, "Reading files")
	processedBytes := int64(0)
	currentFile := ""
	currentFileLock := sync.Mutex{}

	go func() {
		for {
			done := atomic.LoadInt64(&processedBytes)
			if done < 0 {
				return
			}

			currentFileLock.Lock()
			file := currentFile
			currentFileLock.Unlock()

			api.SetProgress(ctx, float32(done)/float32(totalBytes), file)
			time.Sleep(300 * time.Millisecond)
		}
	}()

	missing := make(map[string]map[string][]string)
	invalid := make(map[string]map[string][]string)
	buffer := make([]byte, 128*1024)
	hasher := sha256.New()

	for _, pkg := range rel.Packages {
		for _, archive := range pkg.Archives {
			for _, file := range checksums.Archives[archive.Label].Files {
				fpath := path.Join(pkg.Folder, archive.Destination, file.Filename)

				currentFileLock.Lock()
				currentFile = fpath
				currentFileLock.Unlock()

				f, err := os.Open(filepath.Join(modFolder, fpath))
				if err != nil {
					if eris.Is(err, os.ErrNotExist) {
						if _, ok := missing[pkg.Name]; !ok {
							missing[pkg.Name] = make(map[string][]string)
						}

						missing[pkg.Name][archive.Label] = append(missing[pkg.Name][archive.Label], file.Filename)
						api.Log(ctx, api.LogInfo, "%s is missing", fpath)
						continue
					} else {
						return eris.Wrapf(err, "failed to check %s", fpath)
					}
				}

				hasher.Reset()
				for {
					read, err := f.Read(buffer)
					if err != nil {
						if eris.Is(err, io.EOF) {
							break
						}

						f.Close()
						return eris.Wrapf(err, "failed to read %s", fpath)
					}

					hasher.Write(buffer[0:read])

					if file.Size > 0 {
						atomic.AddInt64(&processedBytes, int64(read))
					}
				}

				if !bytes.Equal(hasher.Sum(nil), file.Checksum) {
					if _, ok := invalid[pkg.Name]; !ok {
						invalid[pkg.Name] = make(map[string][]string)
					}

					invalid[pkg.Name][archive.Label] = append(invalid[pkg.Name][archive.Label], file.Filename)
					api.Log(ctx, api.LogInfo, "%s is corrupted", fpath)
				}
			}
		}
	}

	atomic.StoreInt64(&processedBytes, -1)
	api.Log(ctx, api.LogInfo, "Done")
	api.SetProgress(ctx, 1, "Done")
	return nil
}
