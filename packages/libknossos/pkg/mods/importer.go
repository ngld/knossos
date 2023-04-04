package mods

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aidarkhanov/nanoid"
	"github.com/rotisserie/eris"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/ngld/knossos/packages/api/client"
	"github.com/ngld/knossos/packages/api/common"
	"github.com/ngld/knossos/packages/libknossos/pkg/api"
	"github.com/ngld/knossos/packages/libknossos/pkg/storage"
)

type KnDep struct {
	ID       string
	Version  string
	Packages []string
}

type KnExe struct {
	File       string
	Label      string
	Properties struct {
		X64  bool
		SSE2 bool
		AVX  bool
		AVX2 bool
	}
}

type KnChecksum [2]string

type KnArchive struct {
	Checksum KnChecksum
	Filename string
	Dest     string
	URLs     []string
	FileSize int
}

type KnFile struct {
	Filename string
	Archive  string
	OrigName string
	Checksum KnChecksum
}

type KnPackage struct {
	Name         string
	Notes        string
	Status       string
	Environment  string
	Folder       string
	Dependencies []KnDep
	Executables  []KnExe
	Files        []KnArchive
	Filelist     []KnFile
	IsVp         bool
}

type KnMod struct {
	LocalPath     string
	Title         string
	Version       string
	Parent        string
	Stability     string
	Description   string
	Logo          string
	Tile          string
	Banner        string
	ReleaseThread string `json:"release_thread"`
	Type          string
	ID            string
	Notes         string
	Folder        string
	FirstRelease  string `json:"first_release"`
	LastUpdate    string `json:"last_update"`
	Cmdline       string
	ModFlag       []string `json:"mod_flag"`
	DevMode       bool     `json:"dev_mode"`
	Screenshots   []string
	Packages      []KnPackage
	Videos        []string
}

type UserModSettings struct {
	Cmdline     string
	CustomBuild string `json:"custom_build"`
	LastPlayed  string `json:"last_played"`
	Exe         []string
}

func convertPath(ctx context.Context, modPath, input string) *common.FileRef {
	if input == "" {
		return nil
	}

	ref := &common.FileRef{
		Fileid: "local_" + nanoid.New(),
		Urls:   []string{"file://" + filepath.ToSlash(filepath.Join(modPath, input))},
	}
	err := storage.ImportFile(ctx, ref)
	if err != nil {
		api.Log(ctx, api.LogError, "Failed to import file %s from %s: %s", input, modPath, eris.ToString(err, true))
	}

	return ref
}

func convertChecksum(input KnChecksum) (*common.Checksum, error) {
	digest, err := hex.DecodeString(input[1])
	if err != nil {
		return nil, eris.Wrapf(err, "failed to decode checksum %s", input[1])
	}

	return &common.Checksum{
		Algo:   input[0],
		Digest: digest,
	}, nil
}

func cleanEmptyFolders(folder string) error {
	items, err := os.ReadDir(folder)
	if err != nil {
		return eris.Wrapf(err, "failed to list contents of %s", folder)
	}

	for _, item := range items {
		if item.IsDir() {
			err = cleanEmptyFolders(filepath.Join(folder, item.Name()))
			if err != nil {
				return err
			}
		}
	}

	// Check again because the previous loop might have deleted all remaining folders
	items, err = os.ReadDir(folder)
	if err != nil {
		return eris.Wrapf(err, "failed to list again contents of %s", folder)
	}

	if len(items) == 0 {
		err = os.Remove(folder)
		if err != nil {
			return eris.Wrapf(err, "failed to remove folder %s", folder)
		}
	}

	return nil
}

func ImportMods(ctx context.Context, modFiles []string) error {
	releases := make([]*common.Release, 0)

	api.Log(ctx, api.LogInfo, "Parsing mod.json files")
	api.SetProgress(ctx, 0, "Processing mods")
	modCount := float32(len(modFiles))
	seenMods := make(map[string]bool)

	err := storage.ImportMods(ctx, func(ctx context.Context) error {
		done := float32(0)
		for _, modFile := range modFiles {
			data, err := ioutil.ReadFile(modFile)
			if err != nil {
				return eris.Wrapf(err, "failed to read file %s", modFile)
			}

			var mod KnMod
			err = json.Unmarshal(data, &mod)
			if err != nil {
				return eris.Wrapf(err, "failed to parse contents of %s", modFile)
			}

			modPath, err := filepath.Abs(filepath.Dir(modFile))
			if err != nil {
				return eris.Wrapf(err, "failed build absolute paths to %s", modFile)
			}

			api.SetProgress(ctx, done/modCount, mod.Title+" "+mod.Version)
			done++

			// TODO unnest
			//nolint:nestif
			if !mod.DevMode {
				api.Log(ctx, api.LogInfo, "Converting folder structure to dev mode for %s %s", mod.Title, mod.Version)
				workPath := filepath.Join(modPath, "__dev_work")
				err := os.Mkdir(workPath, 0o770)
				if err != nil {
					return eris.Wrapf(err, "failed to create working directory %s", workPath)
				}

				items, err := os.ReadDir(modPath)
				if err != nil {
					return eris.Wrapf(err, "failed to read contents of directory %s", modPath)
				}

				// Move all items in the mod into the work subfolder to avoid conflicts between package folders and already
				// existing folders. For example, a package folder named "data" containing a data directory would require
				// this case.
				for _, item := range items {
					if item.Name() != "__dev_work" {
						src := filepath.Join(modPath, item.Name())
						dest := filepath.Join(workPath, item.Name())
						err = os.Rename(src, dest)
						if err != nil {
							return eris.Wrapf(err, "failed to rename %s to %s", src, dest)
						}
					}
				}

				for _, pkg := range mod.Packages {
					pkgPath := filepath.Join(modPath, pkg.Folder)
					err = os.Mkdir(pkgPath, 0o770)
					if err != nil && !eris.Is(err, os.ErrExist) {
						return eris.Wrapf(err, "failed to create folder for package %s (%s)", pkg.Name, pkg.Folder)
					}

					for _, pkgFile := range pkg.Filelist {
						src := filepath.Join(workPath, pkgFile.Filename)
						dest := filepath.Join(pkgPath, pkgFile.Filename)
						destParent := filepath.Dir(dest)

						err = os.MkdirAll(destParent, 0o770)
						if err != nil {
							relPath, suberr := filepath.Rel(modPath, dest)
							if suberr != nil {
								relPath = dest
							}
							return eris.Wrapf(err, "failed to create folder file %s in package %s", relPath, pkg.Name)
						}

						err = os.Rename(src, dest)
						if err != nil {
							if eris.Is(err, os.ErrNotExist) {
								api.Log(ctx, api.LogWarn, "file %s seems to be missing, please verify the file integrity for %s once the import is done", src, mod.Title)
							} else {
								return eris.Wrapf(err, "failed to move %s to %s", src, dest)
							}
						}
					}
				}

				api.Log(ctx, api.LogInfo, "Cleaning up")
				err = cleanEmptyFolders(workPath)
				if err != nil {
					return eris.Wrap(err, "failed cleanup")
				}

				leftOvers, err := os.ReadDir(workPath)
				if err != nil {
					if !eris.Is(err, os.ErrNotExist) {
						return eris.Wrap(err, "failed to check work folder")
					}
				} else {
					// Move left overs back to mod folder and remove work folder
					for _, item := range leftOvers {
						src := filepath.Join(workPath, item.Name())
						dest := filepath.Join(modPath, item.Name())
						err = os.Rename(src, dest)
						if err != nil {
							return eris.Wrapf(err, "failed to move %s back to %s", src, dest)
						}
					}

					err = os.Remove(workPath)
					if err != nil {
						return eris.Wrapf(err, "failed to remove work folder %s", workPath)
					}
				}

				data = bytes.Replace(data, []byte(`"dev_mode": false,`), []byte(`"dev_mode": true,`), 1)
				err = os.WriteFile(modFile, data, 0o600)
				if err != nil {
					return eris.Wrapf(err, "failed to update dev_mode field in %s", modFile)
				}

				api.Log(ctx, api.LogInfo, "Folder conversion done")
			}

			if !seenMods[mod.ID] {
				seenMods[mod.ID] = true
				pbMod := &common.ModMeta{
					Modid: mod.ID,
					Title: mod.Title,
				}

				switch mod.Type {
				case "mod":
					pbMod.Type = common.ModType_MOD
				case "tc":
					pbMod.Type = common.ModType_TOTAL_CONVERSION
				case "engine":
					pbMod.Type = common.ModType_ENGINE
				case "tool":
					pbMod.Type = common.ModType_TOOL
				case "extension":
					pbMod.Type = common.ModType_EXTENSION
				default:
					pbMod.Type = common.ModType_MOD
				}

				err = SaveLocalMod(ctx, pbMod)
				if err != nil {
					return eris.Wrapf(err, "failed to import mod %s", mod.ID)
				}
			}

			item := new(common.Release)
			item.Modid = mod.ID
			item.Version = mod.Version
			item.Folder = modPath
			item.Description = mod.Description
			item.Teaser = convertPath(ctx, modPath, mod.Tile)
			item.Banner = convertPath(ctx, modPath, mod.Banner)
			item.ReleaseThread = mod.ReleaseThread
			item.Videos = mod.Videos
			item.Notes = mod.Notes
			item.Cmdline = mod.Cmdline
			item.ModOrder = mod.ModFlag

			releases = append(releases, item)

			if mod.FirstRelease != "" {
				releaseDate, err := time.Parse("2006-01-02", mod.FirstRelease)
				if err != nil {
					return eris.Wrapf(err, "failed to parse release date %s", mod.FirstRelease)
				}

				item.Released = &timestamppb.Timestamp{
					Seconds: releaseDate.Unix(),
				}
			}

			if mod.LastUpdate != "" {
				updateDate, err := time.Parse("2006-01-02", mod.LastUpdate)
				if err != nil {
					return eris.Wrapf(err, "failed to parse update date %s", mod.LastUpdate)
				}

				item.Updated = &timestamppb.Timestamp{
					Seconds: updateDate.Unix(),
				}
			}

			if mod.Type == "engine" {
				switch mod.Stability {
				case "stable":
					item.Stability = common.ReleaseStability_STABLE
				case "rc":
					item.Stability = common.ReleaseStability_RC
				case "nightly":
					item.Stability = common.ReleaseStability_NIGHTLY
				}
			}

			for _, screen := range mod.Screenshots {
				item.Screenshots = append(item.Screenshots, convertPath(ctx, modPath, screen))
			}

			item.Packages = make([]*common.Package, len(mod.Packages))
			for pIdx, pkg := range mod.Packages {
				pbPkg := new(common.Package)
				pbPkg.Name = pkg.Name
				pbPkg.Folder = pkg.Folder
				pbPkg.Notes = pkg.Notes
				pbPkg.KnossosVp = pkg.IsVp

				switch pkg.Status {
				case "required":
					pbPkg.Type = common.PackageType_REQUIRED
				case "recommended":
					pbPkg.Type = common.PackageType_RECOMMENDED
				case "optional":
					pbPkg.Type = common.PackageType_OPTIONAL
				}

				// TODO: CpuSpec

				pbPkg.Dependencies = make([]*common.Dependency, len(pkg.Dependencies))
				for dIdx, dep := range pkg.Dependencies {
					pbDep := new(common.Dependency)
					pbDep.Modid = dep.ID
					pbDep.Constraint = dep.Version
					pbDep.Packages = dep.Packages
					pbPkg.Dependencies[dIdx] = pbDep
				}

				pbPkg.Archives = make([]*common.PackageArchive, len(pkg.Files))
				for aIdx, archive := range pkg.Files {
					pbArchive := new(common.PackageArchive)
					pbArchive.Id = archive.Filename
					pbArchive.Label = archive.Filename
					pbArchive.Destination = archive.Dest

					chk, err := convertChecksum(archive.Checksum)
					if err != nil {
						return err
					}
					pbArchive.Checksum = chk
					pbArchive.Filesize = uint64(archive.FileSize)
					pbArchive.Download = &common.FileRef{
						Fileid: "local_" + nanoid.New(),
						Urls:   archive.URLs,
					}

					pbPkg.Archives[aIdx] = pbArchive
				}

				pbPkg.Files = make([]*common.PackageFile, len(pkg.Filelist))
				for fIdx, file := range pkg.Filelist {
					pbFile := new(common.PackageFile)
					pbFile.Path = file.Filename
					pbFile.Archive = file.Archive
					pbFile.ArchivePath = file.OrigName

					chk, err := convertChecksum(file.Checksum)
					if err != nil {
						return err
					}
					pbFile.Checksum = chk
					pbPkg.Files[fIdx] = pbFile
				}

				pbPkg.Executables = make([]*common.EngineExecutable, len(pkg.Executables))
				for eIdx, exe := range pkg.Executables {
					pbExe := new(common.EngineExecutable)
					pbExe.Path = exe.File
					pbExe.Label = exe.Label

					prio := uint32(0)
					// See https://github.com/ngld/old-knossos/blob/1f60d925498c02d3db76a54d3ee20c31b75c5a21/knossos/repo.py#L35-L40
					if exe.Properties.X64 {
						prio += 50
					}
					if exe.Properties.AVX2 {
						prio += 3
					}
					if exe.Properties.AVX {
						prio += 2
					}
					if exe.Properties.SSE2 {
						prio++
					}
					pbExe.Priority = prio
					pbExe.Debug = strings.Contains(strings.ToLower(exe.Label), "debug")
					pbPkg.Executables[eIdx] = pbExe
				}

				item.Packages[pIdx] = pbPkg
			}

			err = SaveLocalModRelease(ctx, item)
			if err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	api.Log(ctx, api.LogInfo, "Building dependency snapshots")
	err = storage.BatchUpdate(ctx, func(ctx context.Context) error {
		for _, rel := range releases {
			snapshot, err := GetDependencySnapshot(ctx, storage.LocalMods, rel)
			if err != nil {
				api.Log(ctx, api.LogError, "failed to build snapshot for %s (%s): %+v", rel.Modid, rel.Version, err)
				continue
			}

			rel.DependencySnapshot = snapshot
			err = SaveLocalModRelease(ctx, rel)
			if err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	api.Log(ctx, api.LogInfo, "Importing user settings")
	return storage.ImportUserSettings(ctx, func(ctx context.Context, importSettings func(string, string, *client.UserSettings) error) error {
		for _, rel := range releases {
			settingsPath := filepath.Join(rel.Folder, "user.json")
			data, err := ioutil.ReadFile(settingsPath)
			if err != nil {
				if !eris.Is(err, os.ErrNotExist) {
					api.Log(ctx, api.LogError, "failed to open %s: %+v", settingsPath, err)
				}
				continue
			}

			var settings UserModSettings
			err = json.Unmarshal(data, &settings)
			if err != nil {
				api.Log(ctx, api.LogError, "failed to parse %s: %+v", settingsPath, err)
				continue
			}

			newSettings := new(client.UserSettings)
			newSettings.Cmdline = settings.Cmdline
			newSettings.CustomBuild = settings.CustomBuild

			if settings.LastPlayed != "" {
				lastPlayed, err := time.Parse("2006-01-02 15:04:05", settings.LastPlayed)
				if err != nil {
					api.Log(ctx, api.LogWarn, "failed to parse last played date in %s: %+v", settingsPath, err)
				} else {
					newSettings.LastPlayed = &timestamppb.Timestamp{
						Seconds: lastPlayed.Unix(),
					}
				}
			}

			if len(settings.Exe) != 0 {
				if len(settings.Exe) != 2 {
					api.Log(ctx, api.LogWarn, "failed to parse selected build in %s: expected two values but found %+v", settingsPath, settings.Exe)
				} else {
					newSettings.EngineOptions = &client.UserSettings_EngineOptions{
						Modid:   settings.Exe[0],
						Version: settings.Exe[1],
					}
				}
			}

			err = importSettings(rel.Modid, rel.Version, newSettings)
			if err != nil {
				return err
			}
		}

		return nil
	})
}
