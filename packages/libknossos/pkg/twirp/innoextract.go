package twirp

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ngld/knossos/packages/api/client"
	"github.com/ngld/knossos/packages/libinnoextract"
	"github.com/ngld/knossos/packages/libknossos/pkg/api"
	"github.com/ngld/knossos/packages/libknossos/pkg/platform"
	"github.com/rotisserie/eris"
)

func (kn *knossosServer) HandleRetailFiles(ctx context.Context, req *client.HandleRetailFilesRequest) (*client.SuccessResponse, error) {
	api.RunTask(ctx, req.Ref, func(ctx context.Context) error {
		api.SetProgress(ctx, 0, "")

		switch req.Op {
		case client.HandleRetailFilesRequest_AUTO_GOG:
			gameFolder, err := platform.DetectGOGInstallation(ctx)
			if err != nil {
				return err
			}

			return copyFromGameFolder(ctx, req.LibraryPath, gameFolder, false)
		case client.HandleRetailFilesRequest_AUTO_STEAM:
			gameFolder, err := platform.DetectSteamInstallation(ctx)
			if err != nil {
				return err
			}

			return copyFromGameFolder(ctx, req.LibraryPath, gameFolder, false)
		case client.HandleRetailFilesRequest_MANUAL_GOG:
			return handleInnoextract(ctx, req.LibraryPath, req.InstallerPath)
		case client.HandleRetailFilesRequest_MANUAL_FOLDER:
			return copyFromGameFolder(ctx, req.LibraryPath, req.InstallerPath, false)
		default:
			return eris.Errorf("unknown retail files operation %v", req.Op)
		}
	})

	return &client.SuccessResponse{Success: true}, nil
}

func copyFromGameFolder(ctx context.Context, libraryPath, gameFolder string, move bool) error {
	if libraryPath == "" {
		return eris.New("got an empty library path")
	}

	api.Log(ctx, api.LogInfo, "Looking for retail files")

	fs2Path := filepath.Join(libraryPath, "FS2")
	err := os.MkdirAll(fs2Path, 0770)
	if err != nil {
		return eris.Wrapf(err, "failed to create %s", fs2Path)
	}

	files, err := os.ReadDir(gameFolder)
	if err != nil {
		return eris.Wrapf(err, "failed to list contents of %s", gameFolder)
	}

	queue := []string{}

	// We'll need all "*.vp"s
	for _, item := range files {
		if strings.HasSuffix(strings.ToLower(item.Name()), ".vp") {
			queue = append(queue, item.Name())
		}
	}

	// We want the cutscenes as well. GOG's installer puts them in data3 for some reason so check there first.
	data3, err := os.ReadDir(filepath.Join(gameFolder, "data3"))
	d3Prefix := "data3"
	if err != nil {
		if !eris.Is(err, os.ErrNotExist) {
			return eris.Wrapf(err, "failed to access %s", filepath.Join(gameFolder, "data3"))
		}

		// Check the data directory instead if we can't find data3
		data3, err = os.ReadDir(filepath.Join(gameFolder, "data"))
		if err != nil {
			return eris.Wrapf(err, "failed to access %s", filepath.Join(gameFolder, "data"))
		}

		d3Prefix = "data"
	}

	for _, item := range data3 {
		if strings.HasSuffix(strings.ToLower(item.Name()), ".mve") {
			queue = append(queue, filepath.Join(d3Prefix, item.Name()))
		}
	}

	// Some cutscenes are in data2 as well...
	data2, err := os.ReadDir(filepath.Join(gameFolder, "data2"))
	if err == nil {
		for _, item := range data2 {
			if strings.HasSuffix(strings.ToLower(item.Name()), ".mve") {
				queue = append(queue, filepath.Join("data2", item.Name()))
			}
		}
	}

	// Finally add the freddocs to the queue if we can find them.
	freddocs, err := os.ReadDir(filepath.Join(gameFolder, "data", "freddocs"))
	if err == nil {
		for _, item := range freddocs {
			if !item.IsDir() {
				queue = append(queue, filepath.Join("data", "freddocs", item.Name()))
			}
		}
	}

	done := float64(0)
	flen := float64(0)
	buf := make([]byte, 4096)

	for _, item := range queue {
		info, err := os.Stat(filepath.Join(gameFolder, item))
		if err != nil {
			return eris.Wrapf(err, "failed to access %s", filepath.Join(gameFolder, item))
		}

		flen += float64(info.Size())
	}

	if move {
		api.Log(ctx, api.LogInfo, "Moving retail files to library folder")
	} else {
		api.Log(ctx, api.LogInfo, "Copying retail files to library folder")
	}

	for _, item := range queue {
		api.SetProgress(ctx, float32(done/flen), item)

		destPath := filepath.Join(fs2Path, item)
		// I don't know if FSO even supports data2 and data3. Just put everything in data, it's cleaner that way.
		if strings.HasPrefix(item, "data2") || strings.HasPrefix(item, "data3") {
			destPath = filepath.Join(fs2Path, "data", item[6:])
		}

		// Make sure the necessary directories exist
		err = os.MkdirAll(filepath.Dir(destPath), 0770)
		if err != nil {
			return eris.Wrapf(err, "failed to create directories for %s", destPath)
		}

		if move {
			err = os.Rename(filepath.Join(gameFolder, item), destPath)
			if err != nil {
				return eris.Wrapf(err, "failed to move %s to %s", item, destPath)
			}

			info, err := os.Stat(destPath)
			if err != nil {
				return eris.Wrapf(err, "failed to access moved file %s", destPath)
			}

			done += float64(info.Size())
		} else {
			dest, err := os.Create(destPath)
			if err != nil {
				return eris.Wrapf(err, "failed to create %s", destPath)
			}

			source, err := os.Open(filepath.Join(gameFolder, item))
			if err != nil {
				dest.Close()
				return eris.Wrapf(err, "failed to open %s", filepath.Join(gameFolder, item))
			}

			read, err := io.CopyBuffer(dest, source, buf)
			if err != nil {
				dest.Close()
				source.Close()
				return eris.Wrapf(err, "failed to copy data from %s to %s", destPath, filepath.Join(gameFolder, item))
			}

			dest.Close()
			source.Close()

			done += float64(read)
		}
	}

	api.Log(ctx, api.LogInfo, "Done")
	api.Log(ctx, api.LogInfo, "========================================")
	api.Log(ctx, api.LogInfo, "Please close this window to continue")
	api.Log(ctx, api.LogInfo, "========================================")

	return nil
}

var logLevelMap = map[libinnoextract.LogLevel]api.LogLevel{
	libinnoextract.LogDebug:   api.LogDebug,
	libinnoextract.LogError:   api.LogError,
	libinnoextract.LogInfo:    api.LogInfo,
	libinnoextract.LogWarning: api.LogWarn,
}

func handleInnoextract(ctx context.Context, libraryPath, installer string) error {
	if libraryPath == "" {
		return eris.New("got an empty library path")
	}

	tempParent := filepath.Join(libraryPath, "temp")
	_ = os.MkdirAll(tempParent, 0770)

	tempFolder, err := os.MkdirTemp(tempParent, "installer-data-")
	if err != nil {
		return eris.Wrapf(err, "failed to create temporary folder in %s", filepath.Join(libraryPath, "temp"))
	}
	defer os.RemoveAll(tempFolder)

	api.Log(ctx, api.LogInfo, "Loading innoextract")

	innoPath := api.ResourcePath(ctx)
	switch runtime.GOOS {
	case "windows":
		innoPath = filepath.Join(innoPath, "libinnoextract.dll")
	case "linux":
		innoPath = filepath.Join(innoPath, "libinnoextract.so")
	case "darwin":
		innoPath = filepath.Join(innoPath, "libinnoextract.dylib")
	default:
		return eris.Errorf("unsupported OS %s", runtime.GOOS)
	}

	err = libinnoextract.LoadLibrary(innoPath)
	if err != nil {
		return eris.Wrapf(err, "failed to load %s", innoPath)
	}

	api.Log(ctx, api.LogInfo, "Extracting installer")
	err = libinnoextract.ExtractInstaller(installer, tempFolder, func(progress float32, message string) {
		api.SetProgress(ctx, progress, message)
	}, func(level libinnoextract.LogLevel, message string) {
		api.Log(ctx, logLevelMap[level], message)
	})
	if err != nil {
		return eris.Wrap(err, "failed to extract installer")
	}

	err = copyFromGameFolder(ctx, libraryPath, tempFolder, true)

	api.Log(ctx, api.LogInfo, "Cleaning up")
	return err
}