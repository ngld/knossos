package twirp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rotisserie/eris"

	"github.com/ngld/knossos/packages/api/client"
	"github.com/ngld/knossos/packages/api/common"
	"github.com/ngld/knossos/packages/libknossos/pkg/api"
	"github.com/ngld/knossos/packages/libknossos/pkg/helpers"
	"github.com/ngld/knossos/packages/libknossos/pkg/mods"
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

		seenMods := map[string]bool{}
		storage.ImportMods(ctx, func(ctx context.Context, importMod func(*common.ModMeta) error, importRelease func(*common.Release) error) error {
			modFolders := make([]string, 0)

			for _, parent := range parentFolders {
				if !parent.IsDir() {
					continue
				}

				parentFolder := filepath.Join(settings.LibraryPath, parent.Name())

				if parent.Name() != "bin" {
					modFolders = append(modFolders, parentFolder)
				}

				subFolders, err := os.ReadDir(parentFolder)
				if err != nil {
					api.Log(ctx, api.LogError, "Failed to read directory %s! (%s)", parentFolder, err)
					fail = true
					continue
				}
				for _, subFolder := range subFolders {
					if subFolder.IsDir() {
						modFolders = append(modFolders, filepath.Join(parentFolder, subFolder.Name()))
					}
				}
			}

			for _, modFolder := range modFolders {
				modPath := filepath.Join(modFolder, "knmod.json")
				modData, err := os.ReadFile(modPath)
				if err != nil {
					if eris.Is(err, os.ErrNotExist) {
						continue
					}

					return eris.Wrapf(err, "failed to read %s", modPath)
				}

				var mod common.ModMeta
				err = json.Unmarshal(modData, &mod)
				if err != nil {
					api.Log(ctx, api.LogError, "Failed to parse %s, skipping the directory! (%s)", modPath, err)
					fail = true
					continue
				}

				if !seenMods[mod.Modid] {
					seenMods[mod.Modid] = true
					err = importMod(&mod)
					if err != nil {
						return eris.Wrapf(err, "failed to import mod %s from %s", mod.Modid, modPath)
					}
				}

				releasePath := filepath.Join(modFolder, "knrelease.json")
				releaseData, err := os.ReadFile(releasePath)
				if err != nil {
					if eris.Is(err, os.ErrNotExist) {
						continue
					}

					return eris.Wrapf(err, "failed to read %s", releasePath)
				}

				var rel common.Release
				err = json.Unmarshal(releaseData, &rel)
				if err != nil {
					fail = true
					api.Log(ctx, api.LogError, "Failed to parse %s, skipping the directory! (%s)", releaseData, err)
					continue
				}

				err = importRelease(&rel)
				if err != nil {
					return eris.Wrapf(err, "failed to import release %s %s from %s", rel.Modid, rel.Version, modFolder)
				}
			}

			return nil
		})

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

	return &client.ModInfoResponse{
		Mod:      mod,
		Release:  release,
		Versions: versions,
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

func (kn *knossosServer) LaunchMod(ctx context.Context, req *client.LaunchModRequest) (*client.SuccessResponse, error) {
	mod, err := storage.LocalMods.GetModRelease(ctx, req.Modid, req.Version)
	if err != nil {
		return nil, err
	}

	userSettings, err := storage.GetUserSettingsForMod(ctx, req.Modid, req.Version)
	if err != nil {
		return nil, err
	}

	err = mods.LaunchMod(ctx, mod, userSettings)
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
