package twirp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rotisserie/eris"

	"github.com/ngld/knossos/packages/api/client"
	"github.com/ngld/knossos/packages/api/common"
	"github.com/ngld/knossos/packages/libknossos/pkg/api"
	"github.com/ngld/knossos/packages/libknossos/pkg/fsointerop"
	"github.com/ngld/knossos/packages/libknossos/pkg/helpers"
	"github.com/ngld/knossos/packages/libknossos/pkg/mods"
	"github.com/ngld/knossos/packages/libknossos/pkg/platform"
	"github.com/ngld/knossos/packages/libknossos/pkg/storage"
)

func (kn *knossosServer) ScanLocalMods(ctx context.Context, task *client.TaskRequest) (*client.SuccessResponse, error) {
	api.RunTask(ctx, task.Ref, func(ctx context.Context) error {
		settings, err := storage.GetSettings(ctx)
		if err != nil {
			return eris.Wrap(err, "failed to read settings")
		}

		pathQueue := []string{settings.LibraryPath}
		modFiles := []string{}
		for len(pathQueue) > 0 {
			item := pathQueue[0]
			pathQueue = pathQueue[1:]

			info, err := os.Stat(item)
			if err != nil {
				return eris.Wrapf(err, "failed to read library folder %s", item)
			}

			if !info.IsDir() {
				return eris.Errorf("Tried to scan %s which is not a directory", item)
			}

			subs, err := os.ReadDir(item)
			if err != nil {
				return eris.Wrapf(err, "failed to read contents of %s", item)
			}
			for _, entry := range subs {
				if entry.IsDir() {
					pathQueue = append(pathQueue, filepath.Join(item, entry.Name()))
				} else if entry.Name() == "mod.json" {
					modFiles = append(modFiles, filepath.Join(item, "mod.json"))
				}
			}
		}

		api.TaskLog(ctx, client.LogMessage_INFO, "Found %d mod.json files. Importing...", len(modFiles))
		err = mods.ImportMods(ctx, modFiles)
		if err != nil {
			return err
		}

		api.TaskLog(ctx, client.LogMessage_INFO, "Done")
		api.SetProgress(ctx, 1, "Done")
		return nil
	})

	return &client.SuccessResponse{Success: true}, nil
}

func buildModList(ctx context.Context, modProvider storage.ModProvider) (*client.SimpleModList, error) {
	releases, err := modProvider.GetMods(ctx)
	if err != nil {
		return nil, eris.Wrap(err, "failed to load mod list")
	}

	modList := make([]*client.SimpleModList_Item, len(releases))
	modMap := make(map[string]*common.ModMeta)

	for idx, rel := range releases {
		modInfo, found := modMap[rel.Modid]
		if !found {
			modInfo, err = modProvider.GetMod(ctx, rel.Modid)
			if err != nil {
				return nil, eris.Wrapf(err, "failed to load mod %s from storage", rel.Modid)
			}

			modMap[rel.Modid] = modInfo
		}

		modList[idx] = &client.SimpleModList_Item{
			Modid:   rel.Modid,
			Type:    modInfo.Type,
			Title:   modInfo.Title,
			Teaser:  rel.Teaser,
			Version: rel.Version,
			Broken:  len(rel.DependencySnapshot) < 1 && modInfo.Type != common.ModType_ENGINE,
		}
	}

	sort.Sort(helpers.SimpleModListItemsByTitle(modList))

	return &client.SimpleModList{
		Mods: modList,
	}, nil
}

func (kn *knossosServer) UpdateLocalModList(ctx context.Context, req *client.TaskRequest) (*client.SuccessResponse, error) {
	api.RunTask(ctx, req.Ref, func(ctx context.Context) error {
		fail := false
		api.Log(ctx, api.LogInfo, "Looking for knmod.json files")

		settings, err := storage.GetSettings(ctx)
		if err != nil {
			return eris.Wrap(err, "failed to load settings")
		}

		parentFolders, err := os.ReadDir(settings.LibraryPath)
		if err != nil {
			return eris.Wrapf(err, "failed to read directory %s", settings.LibraryPath)
		}

		modInfos := []*common.ModMeta{}
		releaseInfos := []*common.Release{}
		for _, parent := range parentFolders {
			if !parent.IsDir() {
				continue
			}

			subDir := filepath.Join(settings.LibraryPath, parent.Name())
			items, err := os.ReadDir(subDir)
			if err != nil {
				return eris.Wrapf(err, "failed to list contents of %s", subDir)
			}

			for _, item := range items {
				if item.IsDir() {
					releasePath := filepath.Join(subDir, item.Name(), "knrelease.json")
					encodedData, err := os.ReadFile(releasePath)
					if err != nil {
						if eris.Is(err, os.ErrNotExist) {
							// Ignore file not found errors
							continue
						}

						return eris.Wrapf(err, "failed to read %s", releasePath)
					}

					var relaseInfo common.Release
					err = json.Unmarshal(encodedData, &relaseInfo)
					if err != nil {
						return eris.Wrapf(err, "failed to parse %s", releasePath)
					}

					releaseInfos = append(releaseInfos, &relaseInfo)
				} else if strings.HasPrefix(item.Name(), "knmod-") && strings.HasSuffix(item.Name(), ".json") {
					modPath := filepath.Join(subDir, item.Name())
					encodedData, err := os.ReadFile(modPath)
					if err != nil {
						return eris.Wrapf(err, "failed to read %s", modPath)
					}

					var modInfo common.ModMeta
					err = json.Unmarshal(encodedData, &modInfo)
					if err != nil {
						return eris.Wrapf(err, "failed to parse %s", modPath)
					}

					modInfos = append(modInfos, &modInfo)
				}
			}
		}

		err = storage.ImportMods(ctx, func(ctx context.Context) error {
			for _, modInfo := range modInfos {
				err := storage.SaveLocalMod(ctx, modInfo)
				if err != nil {
					return err
				}
			}

			for _, releaseInfo := range releaseInfos {
				err := storage.SaveLocalModRelease(ctx, releaseInfo)
				if err != nil {
					return err
				}
			}

			return nil
		})
		if err != nil {
			return eris.Wrap(err, "failed to import mod metadata")
		}

		if fail {
			return eris.New("some files or folders could not be handled correctly; check the previous messages for details")
		}

		return nil
	})

	return &client.SuccessResponse{Success: true}, nil
}

func (kn *knossosServer) GetLocalMods(ctx context.Context, _ *client.NullMessage) (*client.SimpleModList, error) {
	return buildModList(ctx, storage.LocalMods)
}

func (kn *knossosServer) GetModInfo(ctx context.Context, req *client.ModInfoRequest) (*client.ModInfoResponse, error) {
	mod, err := storage.LocalMods.GetMod(ctx, req.Id)
	if err != nil {
		return nil, err
	}

	release, err := storage.LocalMods.GetModRelease(ctx, req.Id, req.Version)
	if err != nil {
		return nil, err
	}

	versions, err := storage.LocalMods.GetVersionsForMod(ctx, req.Id)
	if err != nil {
		return nil, err
	}

	release.Folder = ""
	release.Packages = make([]*common.Package, 0)

	tools := make([]*client.ToolInfo, 0)
	if mod.Type != common.ModType_ENGINE {
		engine, err := mods.GetEngineForMod(ctx, release)
		if err == nil {
			for _, pkg := range mods.FilterUnsupportedPackages(ctx, engine.Packages) {
				for _, exe := range pkg.Executables {
					found := false
					for _, t := range tools {
						if t.Label == exe.Label {
							found = true
							break
						}
					}

					if !found {
						tools = append(tools, &client.ToolInfo{
							Label: exe.Label,
							Id:    engine.Modid,
							Debug: exe.Debug,
							Fred:  strings.Contains(exe.Label, "FRED"),
						})
					}
				}
			}
			sort.SliceStable(tools, func(i, j int) bool {
				return tools[i].Label < tools[j].Label
			})
		} else {
			api.Log(ctx, api.LogWarn, "Could not resolve engine for mod %s (%s): %s", mod.Title, release.Version, eris.ToString(err, true))
		}
	}

	return &client.ModInfoResponse{
		Mod:      mod,
		Release:  release,
		Versions: versions,
		Tools:    tools,
	}, nil
}

func (kn *knossosServer) GetModDependencies(ctx context.Context, req *client.ModInfoRequest) (*client.ModDependencySnapshot, error) {
	mod, err := storage.LocalMods.GetModRelease(ctx, req.Id, req.Version)
	if err != nil {
		return nil, err
	}

	versions := make(map[string]*client.ModDependencySnapshot_ModInfo)
	for modID := range mod.DependencySnapshot {
		localVersions, err := storage.LocalMods.GetVersionsForMod(ctx, modID)
		if err != nil {
			return nil, err
		}

		versions[modID] = &client.ModDependencySnapshot_ModInfo{
			Versions: localVersions,
		}
	}

	return &client.ModDependencySnapshot{
		Dependencies: mod.DependencySnapshot,
		Available:    versions,
	}, nil
}

func (kn *knossosServer) GetModFlags(ctx context.Context, req *client.ModInfoRequest) (*client.FlagInfo, error) {
	mod, err := storage.LocalMods.GetModRelease(ctx, req.Id, req.Version)
	if err != nil {
		return nil, err
	}

	flagInfo, err := mods.GetFlagsForMod(ctx, mod)
	if err != nil {
		return nil, err
	}

	userSettings, err := storage.GetUserSettingsForMod(ctx, req.Id, req.Version)
	if err != nil {
		return nil, err
	}

	cmdline := userSettings.Cmdline
	if cmdline == "" {
		cmdline = mod.Cmdline
	}

	for _, flag := range flagInfo {
		flag.Enabled = strings.Contains(cmdline, flag.Flag)
	}

	return &client.FlagInfo{
		Flags: flagInfo,
	}, nil
}

func (kn *knossosServer) SaveModFlags(ctx context.Context, req *client.SaveFlagsRequest) (*client.SuccessResponse, error) {
	userSettings, err := storage.GetUserSettingsForMod(ctx, req.Modid, req.Version)
	if err != nil {
		return nil, err
	}

	cmdline := make([]string, 0, len(req.Flags)+1)
	for flag, enabled := range req.Flags {
		if enabled {
			cmdline = append(cmdline, flag)
		}
	}

	cmdline = append(cmdline, req.Freeform)
	userSettings.Cmdline = strings.Join(cmdline, " ")
	err = storage.SaveUserSettingsForMod(ctx, req.Modid, req.Version, userSettings)
	if err != nil {
		return nil, err
	}

	return &client.SuccessResponse{Success: true}, nil
}

func (kn *knossosServer) ResetModFlags(ctx context.Context, req *client.ModInfoRequest) (*client.FlagInfo, error) {
	mod, err := storage.LocalMods.GetModRelease(ctx, req.Id, req.Version)
	if err != nil {
		return nil, err
	}

	flagInfo, err := mods.GetFlagsForMod(ctx, mod)
	if err != nil {
		return nil, err
	}

	userSettings, err := storage.GetUserSettingsForMod(ctx, req.Id, req.Version)
	if err != nil {
		return nil, err
	}

	userSettings.Cmdline = ""
	err = storage.SaveUserSettingsForMod(ctx, req.Id, req.Version, userSettings)
	if err != nil {
		return nil, err
	}

	for _, flag := range flagInfo {
		flag.Enabled = strings.Contains(mod.Cmdline, flag.Flag)
	}

	return &client.FlagInfo{
		Flags: flagInfo,
	}, nil
}

func (kn *knossosServer) LaunchMod(ctx context.Context, req *client.LaunchModRequest) (*client.SuccessResponse, error) {
	mod, err := storage.LocalMods.GetModRelease(ctx, req.Modid, req.Version)
	if err != nil {
		return nil, err
	}

	userSettings, err := storage.GetUserSettingsForMod(ctx, req.Modid, req.Version)
	if err != nil {
		return nil, err
	}

	err = mods.LaunchMod(ctx, mod, userSettings, req.Label)
	if err != nil {
		return nil, err
	}

	return &client.SuccessResponse{Success: true}, nil
}

func (kn *knossosServer) DepSnapshotChange(ctx context.Context, req *client.DepSnapshotChangeRequest) (*client.SuccessResponse, error) {
	rel, err := storage.LocalMods.GetModRelease(ctx, req.Modid, req.Version)
	if err != nil {
		return nil, err
	}

	_, ok := rel.DependencySnapshot[req.DepModid]
	if !ok {
		return nil, eris.Errorf("could not find dependency %s in mod %s %s", req.DepModid, req.Modid, req.Version)
	}

	rel.DependencySnapshot[req.DepModid] = req.DepVersion
	err = storage.SaveLocalModRelease(ctx, rel)
	if err != nil {
		return nil, err
	}

	// TODO: Add warning about potential conflicts (if we detect some).

	return &client.SuccessResponse{Success: true}, nil
}

func (kn *knossosServer) OpenDebugLog(ctx context.Context, req *client.NullMessage) (*client.TaskResult, error) {
	prefPath := fsointerop.GetPrefPath(ctx)
	logPath := filepath.Join(prefPath, "data", "fs2_open.log")

	_, err := os.Stat(logPath)
	if eris.Is(err, os.ErrNotExist) {
		return &client.TaskResult{Error: fmt.Sprintf("Could not find %s", logPath)}, nil
	}

	err = platform.OpenLink(logPath)
	if err != nil {
		return nil, err
	}

	return &client.TaskResult{Success: true}, nil
}

func (kn *knossosServer) VerifyChecksums(ctx context.Context, req *client.VerifyChecksumRequest) (*client.SuccessResponse, error) {
	api.RunTask(ctx, req.Ref, func(ctx context.Context) error {
		rel, err := storage.LocalMods.GetModRelease(ctx, req.Modid, req.Version)
		if err != nil {
			return eris.Wrap(err, "failed to read mod release from storage")
		}

		return mods.VerifyModIntegrity(ctx, rel)
	})

	return &client.SuccessResponse{Success: true}, nil
}
