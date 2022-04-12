package mods

import (
	"bytes"
	"context"
	"crypto/sha256"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
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

	api.Log(ctx, api.LogInfo, "Checking for orphans")
	err = detectOrphans(ctx, rel, checksums, false)
	if err != nil {
		return err
	}

	api.Log(ctx, api.LogInfo, "Done")
	api.SetProgress(ctx, 1, "Done")
	return nil
}

func buildFilelist(dir, prefix string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, eris.Wrapf(err, "failed to list contents of %s", dir)
	}

	result := make([]string, 0)
	for _, item := range entries {
		if item.IsDir() {
			contents, err := buildFilelist(path.Join(dir, item.Name()), path.Join(prefix, item.Name()))
			if err != nil {
				return nil, err
			}

			result = append(result, contents...)
		} else {
			result = append(result, path.Join(prefix, item.Name()))
		}
	}

	return result, nil
}

func detectOrphans(ctx context.Context, rel *common.Release, checksums *common.ChecksumPack, delete bool) error {
	filelist := make(map[string]bool)
	filelist["knrelease.json"] = true

	modFolder, err := GetModFolder(ctx, rel)
	if err != nil {
		return eris.Wrap(err, "failed to build mod folder")
	}

	filerefs := append([]*common.FileRef{rel.Banner, rel.Teaser}, rel.Screenshots...)
	for _, ref := range filerefs {
		if ref != nil && len(ref.Urls) > 0 && strings.HasPrefix(ref.Urls[0], "file://") {
			relPath, err := filepath.Rel(modFolder, ref.Urls[0][7:])
			if err != nil {
				api.Log(ctx, api.LogWarn, "Could not make %s relative to %s", ref.Urls[0][7:], modFolder)
				continue
			}

			filelist[filepath.ToSlash(relPath)] = true
		}
	}

	for _, pkg := range rel.Packages {
		for _, ar := range pkg.Archives {
			arChecksums := checksums.Archives[ar.Label]
			if arChecksums == nil {
				api.Log(ctx, api.LogError, "Checksums for archive %s are missing!", ar.Label)
				continue
			}

			for _, item := range arChecksums.Files {
				itemPath := path.Join(pkg.Folder, ar.Destination, item.Filename)
				filelist[itemPath] = true
			}
		}
	}

	localFiles, err := buildFilelist(modFolder, "")
	if err != nil {
		return eris.Wrapf(err, "failed to build file list for %s", modFolder)
	}

	for _, item := range localFiles {
		if !filelist[item] {
			if delete {
				api.Log(ctx, api.LogInfo, "Deleting orphaned file %s.", item)
				err = os.Remove(filepath.Join(modFolder, filepath.FromSlash(item)))
				if err != nil {
					return eris.Wrapf(err, "failed to remove %s", item)
				}
			} else {
				api.Log(ctx, api.LogWarn, "Found orphaned file %s.", item)
			}
		}
	}

	return nil
}
