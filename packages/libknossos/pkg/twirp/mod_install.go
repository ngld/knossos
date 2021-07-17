package twirp

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"hash"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/davecgh/go-spew/spew"
	"github.com/ngld/knossos/packages/api/client"
	"github.com/ngld/knossos/packages/api/common"
	"github.com/ngld/knossos/packages/libarchive"
	"github.com/ngld/knossos/packages/libknossos/pkg/api"
	"github.com/ngld/knossos/packages/libknossos/pkg/downloader"
	"github.com/ngld/knossos/packages/libknossos/pkg/mods"
	"github.com/ngld/knossos/packages/libknossos/pkg/storage"
	"github.com/rotisserie/eris"
)

func (kn *knossosServer) GetModInstallInfo(ctx context.Context, req *client.ModInfoRequest) (*client.InstallInfoResponse, error) {
	release, err := storage.GetRemoteModRelease(ctx, req.Id, req.Version)
	if err != nil {
		return nil, err
	}
	release.Packages = mods.FilterUnsupportedPackages(ctx, release.Packages)

	// Remote mods don't have a dependency snpashot so we'll have to create a new snapshot
	snapshot, err := mods.GetDependencySnapshot(ctx, storage.RemoteMods{}, release)
	if err != nil {
		return nil, err
	}

	mod, err := storage.GetRemoteMod(ctx, req.Id)
	if err != nil {
		return nil, err
	}

	result := client.InstallInfoResponse{
		Title: mod.Title,
		Mods:  make([]*client.InstallInfoResponse_ModInfo, 1, len(snapshot)+1),
	}

	result.Mods[0] = &client.InstallInfoResponse_ModInfo{
		Id:       mod.Modid,
		Title:    mod.Title,
		Version:  req.Version,
		Notes:    release.Notes,
		Packages: make([]*client.InstallInfoResponse_Package, len(release.Packages)),
	}

	for idx, pkg := range release.Packages {
		pbPkg := &client.InstallInfoResponse_Package{
			Name:         pkg.Name,
			Type:         pkg.Type,
			Notes:        pkg.Notes,
			Dependencies: make([]*client.InstallInfoResponse_Dependency, 0, len(pkg.Dependencies)),
		}
		result.Mods[0].Packages[idx] = pbPkg

		for _, dep := range pkg.Dependencies {
			for _, pkgName := range dep.Packages {
				pbPkg.Dependencies = append(pbPkg.Dependencies, &client.InstallInfoResponse_Dependency{
					Id:      dep.Modid,
					Package: pkgName,
				})
			}
		}
	}

	for modid, version := range snapshot {
		mod, err := storage.GetRemoteMod(ctx, modid)
		if err != nil {
			return nil, err
		}

		rel, err := storage.GetRemoteModRelease(ctx, modid, version)
		if err != nil {
			return nil, err
		}
		rel.Packages = mods.FilterUnsupportedPackages(ctx, rel.Packages)

		modInfo := &client.InstallInfoResponse_ModInfo{
			Id:       mod.Modid,
			Title:    mod.Title,
			Version:  version,
			Notes:    rel.Notes,
			Packages: make([]*client.InstallInfoResponse_Package, len(rel.Packages)),
		}
		result.Mods = append(result.Mods, modInfo)

		for idx, pkg := range rel.Packages {
			modInfo.Packages[idx] = &client.InstallInfoResponse_Package{
				Name:         pkg.Name,
				Type:         pkg.Type,
				Notes:        pkg.Notes,
				Dependencies: make([]*client.InstallInfoResponse_Dependency, 0, len(pkg.Dependencies)),
			}

			for _, dep := range pkg.Dependencies {
				for _, pkgName := range dep.Packages {
					modInfo.Packages[idx].Dependencies = append(modInfo.Packages[idx].Dependencies, &client.InstallInfoResponse_Dependency{
						Id:      dep.Modid,
						Package: pkgName,
					})
				}
			}
		}
	}

	return &result, nil
}

type ModInstallStep struct {
	folder      string
	label       string
	destination string
	modInfo     *common.ModMeta
	relInfo     *common.Release
	pkgInfo     *common.Package
	files       []*common.ChecksumPack_Archive_File
}

func (kn *knossosServer) InstallMod(ctx context.Context, req *client.InstallModRequest) (*client.SuccessResponse, error) {
	api.RunTask(ctx, req.Ref, func(ctx context.Context) error {
		api.Log(ctx, api.LogInfo, "Collecting checksum information")

		plan := make(map[string]ModInstallStep)
		modVersions := make(map[string]string)
		for _, mod := range req.Mods {
			modVersions[mod.Modid] = mod.Version
		}

		info, err := mods.FetchModChecksums(ctx, modVersions)
		if err != nil {
			return err
		}

		checksumLookup := make(map[string]*common.ChecksumPack_Archive)
		for modID, chkInfo := range info {
			for name, ar := range chkInfo.Archives {
				checksumLookup[modID+"#"+name] = ar
			}
		}

		settings, err := storage.GetSettings(ctx)
		if err != nil {
			return err
		}

		tempFolder, err := os.MkdirTemp(filepath.Join(settings.LibraryPath, "temp"), "mod-install")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tempFolder)

		dlItems := make([]*downloader.QueueItem, 0)
		for _, mod := range req.Mods {
			modMeta, err := storage.GetRemoteMod(ctx, mod.Modid)
			if err != nil {
				return err
			}

			relMeta, err := storage.GetRemoteModRelease(ctx, mod.Modid, mod.Version)
			if err != nil {
				return err
			}

			for _, pkg := range relMeta.Packages {
				found := false
				// Check if the user selected this package
				for _, name := range mod.Packages {
					if name == pkg.Name {
						found = true
						break
					}
				}

				if !found {
					continue
				}

				for idx, ar := range pkg.Archives {
					chkInfo, ok := checksumLookup[mod.Modid+"#"+ar.Label]
					if !ok {
						return eris.Errorf("failed to find checksum info for archive %s on mod %s (%s)", ar.Label, modMeta.Title, mod.Modid)
					}

					arPath := filepath.Join(tempFolder, fmt.Sprintf("%s-%s-%d", mod.Modid, pkg.Name, idx))
					arKey := mod.Modid + "#" + pkg.Name + "#" + ar.Label
					dlItems = append(dlItems, &downloader.QueueItem{
						Key:      arKey,
						Filepath: arPath,
						Filesize: int64(chkInfo.Size),
						Mirrors:  chkInfo.Mirrors,
						Checksum: chkInfo.Checksum,
					})

					parent := modMeta.Parent
					if modMeta.Type == common.ModType_ENGINE {
						parent = "bin"
					} else if parent == "" {
						parent = "FS2"
					}

					plan[arKey] = ModInstallStep{
						folder:      filepath.Join(settings.LibraryPath, parent, mod.Modid+"-"+relMeta.Version, pkg.Folder),
						destination: ar.Destination,
						label:       ar.Label,
						modInfo:     modMeta,
						relInfo:     relMeta,
						pkgInfo:     pkg,
						files:       chkInfo.Files,
					}
				}
			}
		}

		stepCount := len(plan)
		done := uint32(0)

		queue := downloader.NewQueue(dlItems)
		queue.ProgressCb = func(progress float32, speed float64) {
			done := atomic.LoadUint32(&done)
			progress += float32(done) / float32(stepCount)

			api.SetProgress(ctx, progress/2, api.FormatBytes(speed)+"/s")
		}

		api.Log(ctx, api.LogInfo, "Starting download")
		// Any error returned here is later checked through queue.Error()
		go queue.Run(ctx) // nolint: errcheck

		hasher := sha256.New()
		buffer := make([]byte, 4096)
		for queue.NextResult() {
			item := queue.Result()
			step := plan[item.Key]

			api.Log(ctx, api.LogInfo, "Opening archive %s for %s", step.label, step.modInfo.Title)
			err = handleArchive(ctx, item.Filepath, &step, hasher, buffer)
			if err != nil {
				return err
			}

			atomic.AddUint32(&done, 1)
		}

		err = queue.Error()
		if err != nil {
			return err
		}

		api.Log(ctx, api.LogInfo, "Done")
		return nil
	})
	return &client.SuccessResponse{Success: true}, nil
}

func handleArchive(ctx context.Context, archivePath string, step *ModInstallStep, hasher hash.Hash, buffer []byte) error {
	fileLookup := make(map[string][]byte)
	for _, item := range step.files {
		fileLookup[strings.TrimPrefix(item.Filename, "./")] = item.Checksum
	}

	archive, err := libarchive.OpenArchive(archivePath)
	if err != nil {
		return err
	}
	defer archive.Close()

	done := make(map[string]bool)
	for {
		err := archive.Next()
		if err != nil {
			if eris.Is(err, io.EOF) {
				break
			}
			return eris.Wrapf(err, "failed to parse archive %s for %s", step.label, step.modInfo.Title)
		}

		// We only care about files
		if archive.Entry.Mode&os.ModeType != 0 {
			continue
		}

		if archive.Entry.Size == 0 {
			api.Log(ctx, api.LogWarn, "Skipping empty file %s", archive.Entry.Pathname)
			continue
		}

		itemName := path.Join(step.destination, archive.Entry.Pathname)
		isDone := done[itemName]
		if isDone {
			api.Log(ctx, api.LogWarn, "Skipping duplicate file %s", archive.Entry.Pathname)
			continue
		}

		done[itemName] = true

		checksum, ok := fileLookup[itemName]
		if !ok {
			spew.Dump(fileLookup)
			return eris.Errorf("could not find checksum for %s in %s for %s", archive.Entry.Pathname, step.label, step.modInfo.Title)
		}

		hasher.Reset()

		destPath := filepath.Join(step.folder, filepath.FromSlash(itemName))
		err = os.MkdirAll(filepath.Dir(destPath), 0770)
		if err != nil {
			return eris.Wrapf(err, "failed to create %s for %s in %s", destPath, step.pkgInfo.Name, step.modInfo.Title)
		}

		// api.Log(ctx, api.LogInfo, "Writing %s", destPath)
		f, err := os.Create(destPath)
		if err != nil {
			return eris.Wrapf(err, "failed to open %s for %s in %s", destPath, step.pkgInfo.Name, step.modInfo.Title)
		}

		for {
			n, readErr := archive.Read(buffer)
			if n > 0 {
				_, err = f.Write(buffer[:n])
				if err != nil {
					f.Close()
					return eris.Wrapf(err, "failed to write %s for %s in %s", destPath, step.pkgInfo.Name, step.modInfo.Title)
				}

				hasher.Write(buffer[:n])
			}

			if readErr != nil {
				if eris.Is(readErr, io.EOF) {
					break
				}
				f.Close()
				return eris.Wrapf(err, "failed to read %s from %s in %s", archive.Entry.Pathname, step.label, step.modInfo.Title)
			}
		}

		err = f.Close()
		if err != nil {
			return eris.Wrapf(err, "failed to close %s for %s in %s", destPath, step.pkgInfo.Name, step.modInfo.Title)
		}

		writtenSum := hasher.Sum(nil)
		if !bytes.Equal(checksum, writtenSum) {
			/*err = os.Remove(destPath)
			if err != nil {
				return eris.Wrapf(err, "failed to remove %s after a checksum error for %s in %s", destPath, step.pkgInfo.Name, step.modInfo.Title)
			}*/

			return eris.Errorf("checksum error (%x != %x) in %s for %s in %s", writtenSum, checksum, archive.Entry.Pathname, step.pkgInfo.Name, step.modInfo.Title)
		}
	}
	return nil
}
