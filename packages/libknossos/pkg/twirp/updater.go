package twirp

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	pbapi "github.com/ngld/knossos/packages/api/api"
	"github.com/ngld/knossos/packages/api/client"
	"github.com/ngld/knossos/packages/libarchive"
	"github.com/ngld/knossos/packages/libknossos/pkg/api"
	"github.com/ngld/knossos/packages/libknossos/pkg/downloader"
	"github.com/rotisserie/eris"
)

func getNebulaAPI() pbapi.Nebula {
	return pbapi.NewNebulaProtobufClient(strings.TrimSuffix(api.TwirpEndpoint, "/"), http.DefaultClient)
}

func (kn *knossosServer) CheckForProgramUpdates(ctx context.Context, req *client.NullMessage) (*client.UpdaterInfoResult, error) {
	resp, err := getNebulaAPI().GetVersions(ctx, &pbapi.NullRequest{})
	if err != nil {
		return nil, eris.Wrap(err, "failed to send version request to nebula")
	}

	updaterFolder := filepath.Join(api.ResourcePath(ctx), "updater")
	_, err = os.Stat(updaterFolder)
	updaterNewVersion := ""

	if eris.Is(err, os.ErrNotExist) {
		api.Log(ctx, api.LogWarn, "Could not find updater folder!")
		updaterNewVersion = resp.Versions["updater"]
	} else {
		updaterVersion, err := os.ReadFile(filepath.Join(updaterFolder, "version.txt"))
		if err != nil {
			if eris.Is(err, os.ErrNotExist) {
				updaterNewVersion = resp.Versions["updater"]
				api.Log(ctx, api.LogWarn, "Updater folder exists but version file is missing!")
			} else {
				api.Log(ctx, api.LogError, "Failed to read from version file: %+v", err)
			}
		} else if resp.Versions["updater"] != string(updaterVersion) {
			updaterNewVersion = resp.Versions["updater"]
		}
	}

	knNewVersion := ""
	if resp.Versions["knossos"] != fmt.Sprintf("%s+%s", api.Version, api.Commit) {
		knNewVersion = resp.Versions["knossos"]
	}

	return &client.UpdaterInfoResult{
		Updater: updaterNewVersion,
		Knossos: knNewVersion,
	}, nil
}

func (kn *knossosServer) UpdateUpdater(ctx context.Context, req *client.TaskRequest) (*client.SuccessResponse, error) {
	api.RunTask(ctx, req.Ref, func(ctx context.Context) error {
		api.Log(ctx, api.LogInfo, "Retrieving updater version")

		resp, err := getNebulaAPI().GetVersions(ctx, &pbapi.NullRequest{})
		if err != nil {
			return eris.Wrap(err, "failed to fetch versions")
		}

		updaterFolder := filepath.Join(api.ResourcePath(ctx), "updater")
		err = os.MkdirAll(updaterFolder, 0770)
		if err != nil {
			return eris.Wrapf(err, "failed to create folder %s", updaterFolder)
		}

		dlURL := fmt.Sprintf("https://github.com/ngld/knossos/releases/download/updater-v%s/updater-v%s", resp.Versions["updater"], resp.Versions["updater"])
		switch runtime.GOOS {
		case "windows":
			dlURL += ".zip"
		case "linux":
			dlURL += ".tar.gz"
		case "darwin":
			dlURL += "-macOS.tar.gz"
		}

		api.Log(ctx, api.LogInfo, "Downloading updater")

		updaterArchive := filepath.Join(updaterFolder, "archive")
		err = downloader.DownloadSingle(ctx, updaterArchive, []string{dlURL}, nil, 3, func(progress float32, speed float64) {
			api.SetProgress(ctx, progress, fmt.Sprintf("Downloading updater %s/s", api.FormatBytes(speed)))
		})
		if err != nil {
			return eris.Wrap(err, "failed to download updater")
		}

		api.Log(ctx, api.LogInfo, "Extracting updater")

		archive, err := libarchive.OpenArchive(updaterArchive)
		if err != nil {
			return eris.Wrapf(err, "failed to open %s", updaterArchive)
		}

		totalBytes := int64(0)
		for archive.Next() == nil {
			totalBytes += archive.Entry.Size
		}

		// We can't seek to the beginning so we have to close and re-open the archive
		err = archive.Close()
		if err != nil {
			return eris.Wrapf(err, "failed to close %s", updaterArchive)
		}

		archive, err = libarchive.OpenArchive(updaterArchive)
		if err != nil {
			return eris.Wrapf(err, "failed to open %s", updaterArchive)
		}
		defer archive.Close()

		written := int64(0)
		fileNames := []string{}

		for {
			err = archive.Next()
			if err != nil {
				if eris.Is(err, io.EOF) {
					break
				}
				return eris.Wrap(err, "failed to read entry from updater archive")
			}

			if archive.Entry.Size == 0 {
				continue
			}

			dest := strings.TrimPrefix(archive.Entry.Pathname, "updater/")
			dest = filepath.Join(updaterFolder, filepath.FromSlash(dest))
			fileNames = append(fileNames, dest)

			err = os.MkdirAll(filepath.Dir(dest), 0770)
			if err != nil {
				return eris.Wrapf(err, "failed to create folders for %s", dest)
			}

			f, err := os.Create(dest)
			if err != nil {
				return eris.Wrapf(err, "failed to create %s", dest)
			}

			entryLength, err := io.Copy(f, archive)
			if err != nil {
				return eris.Wrapf(err, "failed to write %s", dest)
			}

			err = f.Close()
			if err != nil {
				return eris.Wrapf(err, "failed to close %s", dest)
			}

			written += entryLength
			api.SetProgress(ctx, float32(written)/float32(totalBytes), "Unpacking updater")
		}

		archive.Close()

		api.Log(ctx, api.LogInfo, "Removing old files")
		empty, err := oldFileCleaner(ctx, updaterFolder, fileNames)
		if err != nil {
			return eris.Wrap(err, "failed to remove old files")
		}

		if empty {
			return eris.New("somehow all updater files were removed")
		}

		err = os.WriteFile(filepath.Join(updaterFolder, "version.txt"), []byte(resp.Versions["updater"]), 0600)
		if err != nil {
			return eris.Wrapf(err, "failed to write %s", filepath.Join(updaterFolder, "version.txt"))
		}

		api.Log(ctx, api.LogInfo, "Done")
		return nil
	})

	return &client.SuccessResponse{Success: true}, nil
}

func oldFileCleaner(ctx context.Context, dir string, allowed []string) (bool, error) {
	contents, err := os.ReadDir(dir)
	if err != nil {
		return false, eris.Wrapf(err, "failed to read contents of %s", dir)
	}

	deleted := 0
	for _, item := range contents {
		itemPath := filepath.Join(dir, item.Name())

		if item.IsDir() {
			empty, err := oldFileCleaner(ctx, itemPath, allowed)
			if err != nil {
				return false, err
			}

			if empty {
				deleted++
			}
		} else {
			idx := sort.SearchStrings(allowed, itemPath)
			if idx >= len(allowed) || allowed[idx] != itemPath {
				api.Log(ctx, api.LogInfo, fmt.Sprintf("Removing %s", itemPath))
				err = os.Remove(itemPath)
				if err != nil {
					return false, eris.Wrapf(err, "failed to delete %s", itemPath)
				}

				deleted++
			}
		}
	}

	if deleted == len(contents) {
		// We deleted everything in this directory which means that it's empty and can be removed as well.
		err := os.Remove(dir)
		if err != nil {
			return false, eris.Wrapf(err, "failed to delete directory %s", dir)
		}

		return true, nil
	}

	return false, nil
}

func (kn *knossosServer) UpdateKnossos(ctx context.Context, req *client.TaskRequest) (*client.SuccessResponse, error) {
	api.RunTask(ctx, req.Ref, func(ctx context.Context) error {
		api.Log(ctx, api.LogInfo, "Launching updater")

		updater := filepath.Join(api.ResourcePath(ctx), "updater", "updater")
		if runtime.GOOS == "windows" {
			updater += ".exe"
		}

		info, err := os.Stat(updater)
		if err != nil {
			return eris.Wrapf(err, "failed to check updater binary at %s", updater)
		}

		// Make sure the file is executable (aka a+x)
		if info.Mode()&0111 != 0111 {
			err = os.Chmod(updater, info.Mode()|0111)
			if err != nil {
				return eris.Wrapf(err, "failed to set execute permissions on %s", updater)
			}
		}

		cmd := exec.Command(updater, "--auto", api.ResourcePath(ctx))
		err = cmd.Start()
		if err != nil {
			return eris.Wrapf(err, "failed to launch updater")
		}

		return nil
	})

	return &client.SuccessResponse{Success: true}, nil
}
