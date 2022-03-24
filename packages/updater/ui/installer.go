package ui

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/ngld/knossos/packages/libarchive"
	"github.com/ngld/knossos/packages/updater/downloader"
	"github.com/ngld/knossos/packages/updater/platform"
	"github.com/rotisserie/eris"
)

func PerformInstallation(folder, version, token string) {
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		if state == stateInstalling {
			state = stateError
		}

		pan := recover()
		if pan != nil {
			Log(LogError, "panic: %+v", pan)
		}
	}()

	if runtime.GOOS == "windows" {
		if (strings.HasPrefix(folder, "C:\\Program Files") || menuShortcut) && !platform.IsElevated() {
			Log(LogInfo, "Elevating")

			desktopShortcutStr := "false"
			menuShortcutStr := "false"

			if desktopShortcut {
				desktopShortcutStr = "true"
			}

			if menuShortcut {
				menuShortcutStr = "true"
			}

			err := platform.RunElevated(filepath.Clean(os.Args[0]), "--auto", folder, selectedVersion, desktopShortcutStr, menuShortcutStr)
			if err == nil {
				// The defer block above is only meant to catch panics.
				//nolint:gocritic
				os.Exit(0)
			}

			Log(LogError, "Failed: %s", err)
			return
		}
	}

	Log(LogInfo, "Installing Knossos %s in %s", version, folder)
	err := os.MkdirAll(folder, 0o770)
	if err != nil {
		Log(LogError, "Failed to create %s: %s", folder, eris.ToString(err, true))
		return
	}

	tmpArchive := filepath.Join(folder, "knossos_dl")
	err = downloader.DownloadVersion(ctx, token, runtime.GOOS+"-"+version, tmpArchive, SetProgress)
	if err != nil {
		Log(LogError, "Failed to download archive: %s", eris.ToString(err, true))
		return
	}

	Log(LogInfo, "Extracting archive")
	archive, err := libarchive.OpenArchive(tmpArchive)
	if err != nil {
		Log(LogError, "Failed to open downloaded archive: %s", eris.ToString(err, true))
		return
	}
	defer archive.Close()

	seenFiles := make([]string, 0)
	total := 0
	done := 0
	for archive.Next() == nil {
		// We only care about files
		if archive.Entry.Mode&os.ModeType != 0 {
			continue
		}

		total++
	}

	// Reset the archive handle
	archive.Close()
	archive, err = libarchive.OpenArchive(tmpArchive)
	if err != nil {
		Log(LogError, "Failed to open downloaded archive: %s", eris.ToString(err, true))
		return
	}

	buffer := make([]byte, 4096)
	for {
		err = archive.Next()
		if err != nil {
			if eris.Is(err, io.EOF) {
				break
			}

			Log(LogError, "Failed to read from archive: %s", eris.ToString(err, true))
			return
		}

		// We only care about files
		if archive.Entry.Mode&os.ModeType != 0 {
			continue
		}

		SetProgress(float32(done)/float32(total), fmt.Sprintf("Extracting %s", archive.Entry.Pathname))
		done++

		if archive.Entry.Size == 0 {
			Log(LogWarn, "Skipping empty file %s", archive.Entry.Pathname)
			continue
		}

		seenFiles = append(seenFiles, archive.Entry.Pathname)
		dest := filepath.Join(folder, filepath.FromSlash(archive.Entry.Pathname))
		err = os.MkdirAll(filepath.Dir(dest), 0o770)
		if err != nil {
			Log(LogError, "Failed to create directory for %s", archive.Entry.Pathname)
			return
		}

		f, err := os.Create(dest)
		if err != nil {
			Log(LogError, "Failed to write %s: %s", archive.Entry.Pathname, eris.ToString(err, true))
			return
		}

		_, err = io.CopyBuffer(f, archive, buffer)
		if err != nil {
			f.Close()
			Log(LogError, "Failed to read %s from archive: %s", archive.Entry.Pathname, eris.ToString(err, true))
			return
		}

		f.Close()
		err = os.Chmod(dest, archive.Entry.Mode)
		if err != nil {
			Log(LogError, "Failed to set attributes on %s: %s", archive.Entry.Pathname, eris.ToString(err, true))
		}
	}
	archive.Close()

	Log(LogInfo, "Removing old files")
	folderQueue := [][2]string{{folder, ""}}
	sort.Strings(seenFiles)

	for len(folderQueue) > 0 {
		folder := folderQueue[0][0]
		prefix := folderQueue[0][1]
		folderQueue = folderQueue[1:]

		items, err := os.ReadDir(folder)
		if err != nil {
			Log(LogError, "Failed to check %s: %s", folder, eris.ToString(err, true))
			return
		}

		for _, item := range items {
			itemPath := filepath.Join(folder, item.Name())
			itemPrefix := path.Join(prefix, item.Name())

			if item.IsDir() {
				folderQueue = append(folderQueue, [2]string{itemPath, itemPrefix})
			} else {
				idx := sort.SearchStrings(seenFiles, itemPrefix)
				if idx >= len(seenFiles) || seenFiles[idx] != itemPrefix {
					Log(LogInfo, "Removing %s", itemPrefix)
					err = os.Remove(itemPath)
					if err != nil {
						Log(LogError, "Failed: %s", eris.ToString(err, true))
						return
					}
				}
			}
		}
	}

	if runtime.GOOS == "windows" {
		if desktopShortcut {
			Log(LogInfo, "Creating Desktop shortcut")

			path := platform.GetDesktopDirectory()
			if path == "" {
				path = filepath.Join(os.Getenv("USERPROFILE"), "Desktop")
				Log(LogError, "Failed to determine desktop directory, assuming %s instead!", path)
			}

			path = filepath.Join(path, "Knossos.lnk")
			if err := platform.CreateShortcut(path, filepath.Join(folder, "Knossos.exe")); err != nil {
				Log(LogError, "Failed to create desktop shortcut %s! %s", path, err)
				return
			}
		}

		if menuShortcut {
			Log(LogInfo, "Creating shortcut in start menu")

			path := platform.GetStartMenuDirectory()
			if path == "" {
				path = filepath.Join(os.Getenv("ALLUSERSPROFILE"), "Microsoft", "Windows", "Start Menu")
				Log(LogError, "Failed to determine start menu directory, assuming %s instead!", path)
			}

			path = filepath.Join(path, "Knossos.lnk")
			if err := platform.CreateShortcut(path, filepath.Join(folder, "Knossos.exe")); err != nil {
				Log(LogError, "Failed to create start menu shortcut %s! %s", path, err)
				return
			}
		}
	}

	Log(LogInfo, "Done")
	state = stateFinish
}
