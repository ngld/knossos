package twirp

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	release, err := storage.RemoteMods.GetModRelease(ctx, req.Id, req.Version)
	if err != nil {
		return nil, err
	}
	release.Packages = mods.FilterUnsupportedPackages(ctx, release.Packages)

	// Remote mods don't have a dependency snpashot so we'll have to create a new snapshot
	snapshot, err := mods.GetDependencySnapshot(ctx, storage.RemoteMods, release)
	if err != nil {
		return nil, err
	}

	mod, err := storage.RemoteMods.GetMod(ctx, req.Id)
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
		mod, err := storage.RemoteMods.GetMod(ctx, modid)
		if err != nil {
			return nil, err
		}

		rel, err := storage.RemoteMods.GetModRelease(ctx, modid, version)
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
		newMeta := make(map[string]*common.ModMeta)
		newRelMeta := make(map[string]*common.Release)
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
			return eris.Wrap(err, "failed to read settings")
		}

		tempFolder := filepath.Join(settings.LibraryPath, "temp")
		err = os.MkdirAll(tempFolder, 0770)
		if err != nil {
			return eris.Wrap(err, "failed to create temp folder")
		}

		tempFolder, err = os.MkdirTemp(tempFolder, "mod-install")
		if err != nil {
			return eris.Wrap(err, "failed to create temp folder")
		}
		defer os.RemoveAll(tempFolder)

		dlItems := make([]*downloader.QueueItem, 0)
		for _, mod := range req.Mods {
			modMeta, err := storage.RemoteMods.GetMod(ctx, mod.Modid)
			if err != nil {
				return eris.Wrapf(err, "failed to read metadata for %s", mod.Modid)
			}

			relMeta, err := storage.RemoteMods.GetModRelease(ctx, mod.Modid, mod.Version)
			if err != nil {
				return eris.Wrapf(err, "failed to read release metadata for %s %s", mod.Modid, mod.Version)
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

				if _, present := newMeta[mod.Modid]; !present {
					newMeta[mod.Modid] = modMeta
				}

				relKey := mod.Modid + "#" + relMeta.Version
				if _, present := newRelMeta[relKey]; !present {
					// Create a copy of the release without packages; we'll add the installed packages later
					relCopy := new(common.Release)
					*relCopy = *relMeta
					relCopy.Packages = make([]*common.Package, 0)
					newRelMeta[relKey] = relCopy
				}

				newRelMeta[relKey].Packages = append(newRelMeta[relKey].Packages, pkg)

				for idx, ar := range pkg.Archives {
					chkInfo, ok := checksumLookup[mod.Modid+"#"+ar.Label]
					if !ok {
						return eris.Errorf("failed to find checksum info for archive %s on mod %s (%s)", ar.Label, modMeta.Title, mod.Modid)
					}

					arPath := filepath.Join(tempFolder, fmt.Sprintf("%s-%s-%d", mod.Modid, pkg.Name, idx))
					arKey := mod.Modid + "#" + pkg.Name + "#" + ar.Label

					parent := modMeta.Parent
					if modMeta.Type == common.ModType_ENGINE {
						parent = "bin"
					} else if parent == "" {
						parent = "FS2"
					}
					modMeta.Parent = parent

					step := ModInstallStep{
						folder:      filepath.Join(settings.LibraryPath, parent, mod.Modid+"-"+relMeta.Version, pkg.Folder),
						destination: ar.Destination,
						label:       ar.Label,
						modInfo:     modMeta,
						relInfo:     relMeta,
						pkgInfo:     pkg,
						files:       chkInfo.Files,
					}

					allFilesExists := true
					for _, item := range step.files {
						info, err := os.Stat(filepath.Join(step.folder, filepath.FromSlash(item.Filename)))
						if eris.Is(err, os.ErrNotExist) {
							allFilesExists = false
							break
						} else if err != nil {
							api.Log(ctx, api.LogError, "Failed to check file %s of mod %s, assuming that it's missing: %s", item.Filename, modMeta.Title, err)
							allFilesExists = false
							break
						}

						if info.Size() != int64(item.Size) {
							// Ignore incomplete files
							allFilesExists = false
							break
						}
					}

					if allFilesExists {
						api.Log(ctx, api.LogInfo, "Skipping package %s: %s because it's already installed.", modMeta.Title, pkg.Name)
					} else {
						dlItems = append(dlItems, &downloader.QueueItem{
							Key:      arKey,
							Filepath: arPath,
							Filesize: int64(chkInfo.Size),
							Mirrors:  chkInfo.Mirrors,
							Checksum: chkInfo.Checksum,
						})

						plan[arKey] = step
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
			return eris.Wrap(err, "failed to download mod archives")
		}

		api.Log(ctx, api.LogInfo, "Updating mod metadata")
		err = storage.BatchUpdate(ctx, func(ctx context.Context) error {
			modMetas := make(map[string]*common.ModMeta)
			for _, mod := range newMeta {
				err = storage.SaveLocalMod(ctx, mod)
				if err != nil {
					return eris.Wrapf(err, "failed to save mod %s", mod.Title)
				}

				modMetas[mod.Modid] = mod
			}

			for _, rel := range newRelMeta {
				// Keep previously installed packages
				oldRel, err := storage.LocalMods.GetModRelease(ctx, rel.Modid, rel.Version)
				if err == nil {
					for _, oldPkg := range oldRel.Packages {
						found := false
						for _, pkg := range rel.Packages {
							if pkg.Name == oldPkg.Name {
								found = true
								break
							}
						}

						if !found {
							rel.Packages = append(rel.Packages, oldPkg)
						}
					}

					// Preserve downloaded files if possible (no server-side changes)
					if oldRel.Banner.GetFileid() == rel.Banner.GetFileid() {
						rel.Banner = oldRel.Banner
					}

					if oldRel.Teaser.GetFileid() == rel.Teaser.GetFileid() {
						rel.Teaser = oldRel.Teaser
					}

					for idx := 0; idx < len(oldRel.Screenshots) && idx < len(rel.Screenshots); idx++ {
						if oldRel.Screenshots[idx].GetFileid() == rel.Screenshots[idx].GetFileid() {
							rel.Screenshots[idx] = oldRel.Screenshots[idx]
						}
					}
				}

				fileRefs := append([]*common.FileRef{rel.Banner, rel.Teaser}, rel.Screenshots...)
				queueItems := make([]*downloader.QueueItem, 0)
				for _, ref := range fileRefs {
					if ref != nil && (len(ref.Urls) != 1 || !strings.HasPrefix(ref.Urls[0], "file://")) {
						urls := ref.Urls
						ext := filepath.Ext(ref.Urls[0])
						dest := "ref_" + hex.EncodeToString([]byte(ref.Fileid)) + ext
						dest = filepath.Join(settings.LibraryPath, modMetas[rel.Modid].Parent, rel.Modid+"-"+rel.Version, dest)
						ref.Urls = []string{"file://" + filepath.ToSlash(dest)}

						// Only download missing images.
						_, err = os.Stat(dest)
						if err != nil {
							queueItems = append(queueItems, &downloader.QueueItem{
								Key:      ref.Fileid,
								Filepath: dest,
								Mirrors:  urls,
								Checksum: nil,
								Filesize: 0,
							})
						}
					}

					err = storage.ImportFile(ctx, ref)
					if err != nil {
						return eris.Wrapf(err, "failed to import file ref %s", ref.Fileid)
					}
				}

				err = downloader.NewQueue(queueItems).Run(ctx)
				if err != nil {
					return eris.Wrap(err, "failed to fetch mod images")
				}

				// Build dependency snapshot based on the passed snapshot.
				// The user-requested mod will receive the full snapshot but dependencies usually only need a subset.
				// For example, FSO's dep snapshot would only contain FSO while the MVPs' snapshot would only contain
				// the MVPs and FSO and any other mods the MVPs might depend on.
				rel.DependencySnapshot, err = mods.GetDependencySnapshot(ctx, storage.RemoteMods, rel)
				if err != nil {
					return eris.Wrapf(err, "failed to build dependency snpashot for %s (%s)", modMetas[rel.Modid].Title, rel.Version)
				}

				for modid := range rel.DependencySnapshot {
					version, ok := modVersions[modid]
					if !ok {
						return eris.Errorf("dependency snapshot for %s (%s) contains %s but it's missing from the initial request", modMetas[rel.Modid].Title, rel.Version, modid)
					}

					// Not sure how problematic this would be, just warn for now.
					if rel.DependencySnapshot[modid] != version {
						api.Log(ctx, api.LogWarn, "Mod %s (%s) would use dependency %s (%s) but the request uses %s",
							modMetas[rel.Modid].Title, rel.Version, modMetas[modid].Title, rel.DependencySnapshot[modid], version)
					}
				}

				err = storage.SaveLocalModRelease(ctx, rel)
				if err != nil {
					return eris.Wrapf(err, "failed to save release %s (%s)", modMetas[rel.Modid].Title, rel.Version)
				}
			}

			return nil
		})
		if err != nil {
			return err
		}

		// TODO save dependency snapshots

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
			err = os.Remove(destPath)
			if err != nil {
				return eris.Wrapf(err, "failed to remove %s after a checksum error for %s in %s", destPath, step.pkgInfo.Name, step.modInfo.Title)
			}

			return eris.Errorf("checksum error (%x != %x) in %s for %s in %s", writtenSum, checksum, archive.Entry.Pathname, step.pkgInfo.Name, step.modInfo.Title)
		}
	}
	return nil
}
