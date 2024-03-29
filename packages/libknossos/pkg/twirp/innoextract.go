package twirp

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/ngld/knossos/packages/api/client"
	"github.com/ngld/knossos/packages/api/common"
	"github.com/ngld/knossos/packages/libknossos/pkg/api"
	"github.com/ngld/knossos/packages/libknossos/pkg/mods"
	"github.com/ngld/knossos/packages/libknossos/pkg/platform"
	"github.com/ngld/knossos/packages/libknossos/pkg/storage"
	"github.com/rotisserie/eris"
	"google.golang.org/protobuf/types/known/timestamppb"
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
	err := os.MkdirAll(fs2Path, 0o770)
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
		err = os.MkdirAll(filepath.Dir(destPath), 0o770)
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

	err = mods.UpdateRemoteModIndex(ctx)
	if err != nil {
		return eris.Wrap(err, "failed to fetch available mods")
	}

	api.SetProgress(ctx, 0, "Installing FSO")
	api.Log(ctx, api.LogInfo, "Installing FSO")
	fsoVersions, err := storage.RemoteMods.GetVersionsForMod(ctx, "FSO")
	if err != nil || len(fsoVersions) < 1 {
		return eris.Wrap(err, "failed to retrieve FSO versions")
	}

	var fsoRel *common.Release
	var packageNames []string
	for idx := len(fsoVersions) - 1; idx >= 0; idx-- {
		fsoRel, err = storage.RemoteMods.GetModRelease(ctx, "FSO", fsoVersions[idx])
		if err != nil {
			return eris.Wrapf(err, "failed to load FSO release %s", fsoVersions[idx])
		}

		// Make sure this release is actually supported on this platform
		fsoRel.Packages = mods.FilterUnsupportedPackages(ctx, fsoRel.Packages)
		packageNames = make([]string, len(fsoRel.Packages))

		for idx, pkg := range fsoRel.Packages {
			packageNames[idx] = pkg.Name
		}

		if len(fsoRel.Packages) > 0 {
			break
		}
	}

	// NOTE: We assume that FSO has no dependencies here. If that changes, we'll have to perform dependency resolution
	// as well which means we should probably refactor the above code as well to make installing mods/packages through
	// the API simpler.
	err = mods.InstallMod(ctx, &client.InstallModRequest{
		Mods: []*client.InstallModRequest_Mod{{Modid: "FSO", Version: fsoRel.Version, Packages: packageNames}},
	})
	if err != nil {
		return eris.Wrap(err, "failed to install FSO")
	}

	err = mods.SaveLocalMod(ctx, &common.ModMeta{
		Modid: "FS2",
		Title: "Retail FS2",
		Type:  common.ModType_TOTAL_CONVERSION,
	})
	if err != nil {
		return eris.Wrap(err, "failed to create Retail FS2 entry")
	}

	releaseDate, err := time.Parse("2006-01-02", "1999-09-30")
	if err != nil {
		panic("How?!")
	}

	updateDate, err := time.Parse("2006-01-02", "1999-12-03")
	if err != nil {
		panic("How?!")
	}

	retailRel := &common.Release{
		Modid:   "FS2",
		Version: "1.20.0",
		Folder:  "FS2",
		Description: strings.ReplaceAll(strings.ReplaceAll(`[b][i]The year is 2367, thirty two years after the Great War. Or at least that is what YOU thought was the Great War.
		The endless line of Shivan capital ships, bombers and fighters with super advanced technology was nearly overwhelming.\n\n
		As the Terran and Vasudan races finish rebuilding their decimated societies, a disturbance lurks in the not-so-far
		reaches of the Gamma Draconis system.\n\nYour nemeses have arrived... and they are wondering what happened to
		their scouting party.[/i][/b]\n\n[hr]FreeSpace 2 is a 1999 space combat simulation computer game developed by Volition as
		the sequel to Descent: FreeSpace – The Great War. It was completed ahead of schedule in less than a year, and
		released to very positive reviews.\n\nThe game continues on the story from Descent: FreeSpace, once again
		thrusting the player into the role of a pilot fighting against the mysterious aliens, the Shivans. While defending
		the human race and its alien Vasudan allies, the player also gets involved in putting down a rebellion. The game
		features large numbers of fighters alongside gigantic capital ships in a battlefield fraught with beams, shells and
		missiles in detailed star systems and nebulae.`, "\n", ""), "\\n", "\n"),
		ReleaseThread: "http://www.hard-light.net/forums/index.php",
		Videos:        []string{"https://www.youtube.com/watch?v=ufViyhrXzTE"},
		Released:      timestamppb.New(releaseDate),
		Updated:       timestamppb.New(updateDate),
		Packages: []*common.Package{{
			Name:   "Content",
			Type:   common.PackageType_REQUIRED,
			Folder: ".",
			Dependencies: []*common.Dependency{{
				Modid:      "FSO",
				Constraint: ">=3.8.0-2",
			}},
		}},
	}

	// Save the release in our storage since mods.GetDependencySnapshot will look it up
	err = storage.SaveLocalModRelease(ctx, retailRel)
	if err != nil {
		return eris.Wrap(err, "failed to save FS2 retail release data")
	}

	retailRel.DependencySnapshot, err = mods.GetDependencySnapshot(ctx, storage.LocalMods, retailRel)
	if err != nil {
		return eris.Wrap(err, "failed to build dependency snapshot for FS2 retail")
	}

	// Now we can save the JSON files as well (which are necessary to see the mod after the next launch)
	err = mods.SaveLocalModRelease(ctx, retailRel)
	if err != nil {
		return eris.Wrap(err, "failed to save FS2 retail release data")
	}

	api.Log(ctx, api.LogInfo, "Done")
	api.Log(ctx, api.LogInfo, "========================================")
	api.Log(ctx, api.LogInfo, "Please close this window to continue")
	api.Log(ctx, api.LogInfo, "========================================")

	return nil
}

func handleInnoextract(ctx context.Context, libraryPath, installer string) error {
	if libraryPath == "" {
		return eris.New("got an empty library path")
	}

	tempParent := filepath.Join(libraryPath, "temp")
	_ = os.MkdirAll(tempParent, 0o770)

	tempFolder, err := os.MkdirTemp(tempParent, "installer-data-")
	if err != nil {
		return eris.Wrapf(err, "failed to create temporary folder in %s", filepath.Join(libraryPath, "temp"))
	}
	defer os.RemoveAll(tempFolder)

	api.Log(ctx, api.LogInfo, "Launching innoextract")

	innoPath := api.ResourcePath(ctx)
	switch runtime.GOOS {
	case "windows":
		innoPath = filepath.Join(innoPath, "innoextract.exe")
	case "linux", "darwin":
		innoPath = filepath.Join(innoPath, "innoextract")
	default:
		return eris.Errorf("unsupported OS %s", runtime.GOOS)
	}

	proc := exec.CommandContext(ctx, innoPath, "-c0", "-p1", installer, "-d", tempFolder)
	stdout, err := proc.StdoutPipe()
	if err != nil {
		return eris.Wrap(err, "failed to allocate stdout pipe")
	}

	stderr, err := proc.StderrPipe()
	if err != nil {
		return eris.Wrap(err, "failed to allocate stderr pipe")
	}

	err = proc.Start()
	if err != nil {
		return eris.Wrapf(err, "failed to run %s", innoPath)
	}

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			api.Log(ctx, api.LogError, "InnoExtract: %s", scanner.Text())
		}
	}()

	scanner := bufio.NewScanner(stdout)
	scanner.Split(func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		start := 0
		bracket := -1
		inEscape := false

	loop:
		for idx := 0; idx < len(data); idx++ {
			if inEscape {
				if data[idx] == 'K' {
					inEscape = false
					start = idx + 1
				}
				continue
			}

			switch data[idx] {
			case '\x1b':
				inEscape = true
			case '\r', '\n':
				if idx == start {
					start++
					continue
				}
				return idx, data[start:idx], nil
			case ']':
				bracket = idx
				break loop
			}
		}

		if bracket == -1 {
			return 0, nil, nil
		}

		data = data[bracket+1:]
		end := bytes.IndexByte(data, '\r')
		if end > 0 {
			data = data[:end]
		} else {
			end = 0
		}

		return bracket + end + 2, data, nil
	})

	for scanner.Scan() {
		parts := strings.SplitN(scanner.Text(), "%", 2)
		parts[0] = strings.TrimSpace(parts[0])

		if len(parts) < 2 {
			if len(parts[0]) > 0 {
				api.Log(ctx, api.LogInfo, "InnoExtract: %s", parts[0])
			}
			continue
		}

		progress, err := strconv.ParseFloat(parts[0], 32)
		if err != nil {
			progress = 0
		}

		label := strings.TrimSpace(parts[1])
		api.SetProgress(ctx, float32(progress)/100, label)
	}

	err = proc.Wait()
	if err != nil {
		return eris.Wrap(err, "failed to run innoextract")
	}

	err = copyFromGameFolder(ctx, libraryPath, tempFolder, true)

	api.Log(ctx, api.LogInfo, "Cleaning up")
	return err
}
