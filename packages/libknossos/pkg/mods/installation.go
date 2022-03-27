package mods

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/ngld/knossos/packages/api/client"
	"github.com/ngld/knossos/packages/api/common"
	"github.com/ngld/knossos/packages/libarchive"
	"github.com/ngld/knossos/packages/libknossos/pkg/api"
	"github.com/ngld/knossos/packages/libknossos/pkg/downloader"
	"github.com/ngld/knossos/packages/libknossos/pkg/storage"
	"github.com/rotisserie/eris"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func handleArchive(ctx context.Context, archivePath string, step *ModInstallStep, hasher hash.Hash, buffer []byte, progress *uint32) error {
	fileLookup := make(map[string][]byte)
	for _, item := range step.files {
		fileLookup[strings.TrimPrefix(item.Filename, "./")] = item.Checksum
	}

	archive, err := libarchive.OpenArchive(archivePath)
	if err != nil {
		return err
	}
	defer archive.Close()

	size := archive.Size()
	progressOffset := atomic.LoadUint32(progress)

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

		if archive.Entry.SymlinkDest != "" {
			if filepath.IsAbs(archive.Entry.SymlinkDest) {
				return eris.Errorf("symlink %s points to the absolute path %s which is not allowed; found in %s for %s", archive.Entry.Pathname, archive.Entry.SymlinkDest, step.pkgInfo.Name, step.modInfo.Title)
			}

			destPath := filepath.Join(step.folder, step.destination, archive.Entry.Pathname)
			err = os.MkdirAll(filepath.Dir(destPath), 0o770)
			if err != nil {
				return eris.Wrapf(err, "failed to create %s for %s in %s", destPath, step.pkgInfo.Name, step.modInfo.Title)
			}

			err = os.Symlink(archive.Entry.SymlinkDest, destPath)
			if err != nil {
				return eris.Wrapf(err, "failed to create symlink %s pointing to %s for %s in %s", destPath, archive.Entry.SymlinkDest, step.pkgInfo.Name, step.modInfo.Title)
			}

			continue
		}

		if archive.Entry.Size == 0 {
			// Skip warning for folders in .zip files.
			if !strings.HasSuffix(archive.Entry.Pathname, "/") {
				api.Log(ctx, api.LogWarn, "Skipping empty file %s", archive.Entry.Pathname)
			}
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
			spew.Dump(fileLookup, itemName)
			return eris.Errorf("could not find checksum for %s in %s for %s", archive.Entry.Pathname, step.label, step.modInfo.Title)
		}

		hasher.Reset()

		destPath := filepath.Join(step.folder, filepath.FromSlash(itemName))
		err = os.MkdirAll(filepath.Dir(destPath), 0o770)
		if err != nil {
			return eris.Wrapf(err, "failed to create %s for %s in %s", destPath, step.pkgInfo.Name, step.modInfo.Title)
		}

		// api.Log(ctx, api.LogInfo, "Writing %s", destPath)
		f, err := os.Create(destPath)
		if err != nil {
			return eris.Wrapf(err, "failed to open %s for %s in %s", destPath, step.pkgInfo.Name, step.modInfo.Title)
		}

		if archive.Entry.Mode&0o700 == 0o700 {
			err = os.Chmod(destPath, 0o777)
			if err != nil {
				return eris.Wrapf(err, "failed to set executable attribute on %s for %s in %s", destPath, step.pkgInfo.Name, step.modInfo.Title)
			}
		}

		for {
			n, readErr := archive.Read(buffer)
			if n > 0 {
				_, err = f.Write(buffer[0:n])
				if err != nil {
					f.Close()
					return eris.Wrapf(err, "failed to write %s for %s in %s", destPath, step.pkgInfo.Name, step.modInfo.Title)
				}

				_, err = hasher.Write(buffer[0:n])
				if err != nil {
					f.Close()
					return eris.Wrapf(err, "failed to hash chunk of %s for %s in %s", destPath, step.pkgInfo.Name, step.modInfo.Title)
				}
			}

			if readErr != nil {
				if eris.Is(readErr, io.EOF) {
					break
				}
				f.Close()
				return eris.Wrapf(err, "failed to read %s from %s in %s", archive.Entry.Pathname, step.label, step.modInfo.Title)
			}

			// Rescale the position to [0-100] range and add it to the progress offset
			pos := archive.Position()
			atomic.StoreUint32(progress, progressOffset+uint32(float32(100*pos)/float32(size)))
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

	// Make sure we're at exactly 100% once we're done
	atomic.StoreUint32(progress, progressOffset+100)
	return nil
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

func InstallMod(ctx context.Context, req *client.InstallModRequest) error {
	api.Log(ctx, api.LogInfo, "Collecting checksum information")

	plan := make(map[string]ModInstallStep)
	modVersions := make(map[string]string)
	newMeta := make(map[string]*common.ModMeta)
	newRelMeta := make(map[string]*common.Release)
	for _, mod := range req.Mods {
		modVersions[mod.Modid] = mod.Version
	}

	info, err := FetchModChecksums(ctx, modVersions)
	if err != nil {
		return err
	}

	api.Log(ctx, api.LogInfo, "Planning mod installation")
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
	err = os.MkdirAll(tempFolder, 0o770)
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

		parent := modMeta.Parent
		switch {
		case modMeta.Type == common.ModType_ENGINE:
			parent = "bin"
		case modMeta.Type == common.ModType_TOTAL_CONVERSION:
			parent = modMeta.Modid
		case parent == "":
			parent = "FS2"
		}

		modMeta.Parent = parent
		modFolder := filepath.Join(settings.LibraryPath, parent, fmt.Sprintf("%s-%s", relMeta.Modid, relMeta.Version))
		err = os.MkdirAll(modFolder, 0o700)
		if err != nil {
			return eris.Wrapf(err, "failed to create mod folder %s", modFolder)
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
				data, err := proto.Marshal(modMeta)
				if err != nil {
					return eris.Wrapf(err, "failed to serialise mod %s", modMeta.Modid)
				}

				newMeta[mod.Modid] = new(common.ModMeta)
				err = proto.Unmarshal(data, newMeta[mod.Modid])
				if err != nil {
					return eris.Wrapf(err, "failed to deserialise mod %s", modMeta.Modid)
				}
			}

			modMeta = newMeta[mod.Modid]
			relKey := mod.Modid + "#" + relMeta.Version
			if _, present := newRelMeta[relKey]; !present {
				// Create a copy of the release without packages; we'll add the installed packages later
				data, err := proto.Marshal(relMeta)
				if err != nil {
					return eris.Wrapf(err, "failed to serialise mod release %s (%s)", modMeta.Modid, relMeta.Version)
				}

				newRelMeta[relKey] = new(common.Release)
				err = proto.Unmarshal(data, newRelMeta[relKey])
				if err != nil {
					return eris.Wrapf(err, "failed to deserialise mod release %s (%s)", modMeta.Modid, relMeta.Version)
				}

				newRelMeta[relKey].Packages = make([]*common.Package, 0)
			}

			newRelMeta[relKey].Packages = append(newRelMeta[relKey].Packages, pkg)

			for idx, ar := range pkg.Archives {
				chkInfo, ok := checksumLookup[mod.Modid+"#"+ar.Label]
				if !ok {
					return eris.Errorf("failed to find checksum info for archive %s on mod %s (%s)", ar.Label, modMeta.Title, mod.Modid)
				}

				arPath := filepath.Join(tempFolder, fmt.Sprintf("%s-%s-%d", mod.Modid, pkg.Name, idx))
				arKey := mod.Modid + "#" + pkg.Name + "#" + ar.Label

				step := ModInstallStep{
					folder:      filepath.Join(modFolder, pkg.Folder),
					destination: ar.Destination,
					label:       ar.Label,
					modInfo:     modMeta,
					relInfo:     relMeta,
					pkgInfo:     pkg,
					files:       chkInfo.Files,
				}

				allFilesExists := true
				for _, item := range step.files {
					itemPath := filepath.Join(step.folder, filepath.FromSlash(item.Filename))
					info, err := os.Stat(itemPath)
					if eris.Is(err, os.ErrNotExist) {
						api.Log(ctx, api.LogDebug, "File %s is missing", itemPath)
						allFilesExists = false
						break
					} else if err != nil {
						api.Log(ctx, api.LogError, "Failed to check file %s of mod %s, assuming that it's missing: %s", item.Filename, modMeta.Title, err)
						allFilesExists = false
						break
					}

					if item.Size > 0 && info.Size() != int64(item.Size) {
						api.Log(ctx, api.LogDebug, "File %s has wrong file size (%d != %d)", itemPath, info.Size(), item.Size)
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

	stepCount := len(plan) * 100
	done := uint32(0)

	queue, err := downloader.NewQueue(ctx, dlItems)
	if err != nil {
		return eris.Wrap(err, "failed to prepare download queue")
	}

	queue.ProgressCb = func(progress float32, speed float64) {
		done := atomic.LoadUint32(&done)
		progress += float32(done) / float32(stepCount)

		api.SetProgress(ctx, progress/2, api.FormatBytes(speed)+"/s")
	}

	api.Log(ctx, api.LogInfo, "Starting download")

	active := true
	defer func() { active = false }()
	go func() {
		defer api.CrashReporter(ctx)

		// Any error returned here is later checked through queue.Error()
		queue.Run(ctx) // nolint: errcheck

		for active {
			done := atomic.LoadUint32(&done)
			progress := float32(done) / float32(stepCount)

			if progress == 1 {
				api.SetProgress(ctx, 1, "Finishing")
				return
			}

			api.SetProgress(ctx, 0.5+(progress/2), "Extracting")
			time.Sleep(300 * time.Millisecond)
		}
	}()

	hasher := sha256.New()
	buffer := make([]byte, 32*1024)
	for queue.NextResult() {
		item := queue.Result()
		step := plan[item.Key]

		api.Log(ctx, api.LogInfo, "Opening archive %s for %s", step.label, step.modInfo.Title)
		err = handleArchive(ctx, item.Filepath, &step, hasher, buffer, &done)
		if err != nil {
			queue.Abort()
			return err
		}
	}

	err = queue.Error()
	if err != nil {
		return eris.Wrap(err, "failed to download mod archives")
	}

	api.Log(ctx, api.LogInfo, "Updating mod metadata")
	modMetas := make(map[string]*common.ModMeta)
	err = storage.BatchUpdate(ctx, func(ctx context.Context) error {
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
			modFolder, err := GetModFolder(ctx, rel)
			if err != nil {
				return eris.Wrapf(err, "failed to build folder path for %s %s", rel.Modid, rel.Version)
			}

			for _, ref := range fileRefs {
				if ref == nil {
					continue
				}

				if len(ref.Urls) != 1 || !strings.HasPrefix(ref.Urls[0], "file://") {
					dest := "ref_" + hex.EncodeToString([]byte(ref.Fileid)) + filepath.Ext(ref.Urls[0])
					dest = filepath.Join(modFolder, dest)
					mirrors := ref.Urls
					ref.Urls = []string{"file://" + filepath.ToSlash(dest)}

					// Only download missing images.
					_, err = os.Stat(dest)
					if err != nil {
						queueItems = append(queueItems, &downloader.QueueItem{
							Key:      ref.Fileid,
							Filepath: dest,
							Mirrors:  mirrors,
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

			dlq, err := downloader.NewQueue(ctx, queueItems)
			if err != nil {
				return eris.Wrap(err, "failed to prepare download queue for mod images")
			}

			err = dlq.Run(ctx)
			if err != nil {
				return eris.Wrap(err, "failed to fetch mod images")
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

	err = storage.BatchUpdate(ctx, func(ctx context.Context) error {
		snapQueue := make([]interface{}, len(newRelMeta)+len(req.SnapshotAfter))
		idx := 0
		for _, rel := range newRelMeta {
			snapQueue[idx] = rel
			idx++
		}

		for _, snap := range req.SnapshotAfter {
			snapQueue[idx] = snap
			idx++
		}

		for _, item := range snapQueue {
			rel, ok := item.(*common.Release)
			if !ok {
				spec, ok := item.(*client.InstallModRequest_Mod)
				if !ok {
					panic("found impossible type in snapQueue")
				}

				rel, err = storage.LocalMods.GetModRelease(ctx, spec.Modid, spec.Version)
				if err != nil {
					return eris.Wrapf(err, "failed to look up mod %s (%s) for snapshotting", spec.Modid, spec.Version)
				}
			}

			// Build dependency snapshot for the given mod release.
			// The user-requested mod will receive the full snapshot but dependencies usually only need a subset.
			// For example, FSO's dep snapshot would only contain FSO while the MVPs' snapshot would only contain
			// the MVPs and FSO and any other mods the MVPs might depend on.
			rel.DependencySnapshot, err = GetDependencySnapshot(ctx, storage.RemoteMods, rel)
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

			modFolder, err := GetModFolder(ctx, rel)
			if err != nil {
				return eris.Wrapf(err, "failed to build folder path for %s %s", rel.Modid, rel.Version)
			}

			// TODO: This should be unnecessary since we've already installed the mod at this point.
			// If the folder doesn't exist, the mod installation must have failed.
			err = os.MkdirAll(modFolder, 0o700)
			if err != nil {
				return eris.Wrapf(err, "failed to create mod folder %s for %s %s", modFolder, rel.Modid, rel.Version)
			}

			releaseData, err := json.MarshalIndent(rel, "", "  ")
			if err != nil {
				return eris.Wrapf(err, "failed to serialise release %s %s", rel.Modid, rel.Version)
			}

			releaseJSON := filepath.Join(modFolder, "knrelease.json")
			err = os.WriteFile(releaseJSON, releaseData, 0o600)
			if err != nil {
				return eris.Wrapf(err, "failed to write %s for %s %s", releaseJSON, rel.Modid, rel.Version)
			}

			modData, err := json.MarshalIndent(modMetas[rel.Modid], "", "  ")
			if err != nil {
				return eris.Wrapf(err, "failed to serialise mod metadata for %s", rel.Modid)
			}

			modJSON := filepath.Join(settings.LibraryPath, modMetas[rel.Modid].Parent, "knmod-"+rel.Modid+".json")
			err = os.WriteFile(modJSON, modData, 0o600)
			if err != nil {
				return eris.Wrapf(err, "failed to write %s for %s %s", modJSON, rel.Modid, rel.Version)
			}

			rel.JsonExportUpdated = timestamppb.Now()
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

	api.Log(ctx, api.LogInfo, "Done")
	return nil
}
