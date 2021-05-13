package twirp

import (
	"context"
	"sort"

	"github.com/ngld/knossos/packages/api/client"
	"github.com/ngld/knossos/packages/api/common"
	"github.com/ngld/knossos/packages/libknossos/pkg/api"
	"github.com/ngld/knossos/packages/libknossos/pkg/helpers"
	"github.com/ngld/knossos/packages/libknossos/pkg/mods"
	"github.com/ngld/knossos/packages/libknossos/pkg/storage"
)

func (kn *knossosServer) SyncRemoteMods(ctx context.Context, task *client.TaskRequest) (*client.SuccessResponse, error) {
	api.RunTask(ctx, task.Ref, mods.UpdateRemoteModIndex)
	return &client.SuccessResponse{Success: true}, nil
}

func (kn *knossosServer) GetRemoteMods(ctx context.Context, _ *client.NullMessage) (*client.SimpleModList, error) {
	releases, err := storage.GetRemoteMods(ctx, 0)
	if err != nil {
		return nil, err
	}

	modList := make([]*client.SimpleModList_Item, len(releases))
	modMap := make(map[string]*common.ModMeta)

	for idx, rel := range releases {
		modInfo, found := modMap[rel.Modid]
		if !found {
			modInfo, err = storage.GetRemoteMod(ctx, rel.Modid)
			if err != nil {
				return nil, err
			}

			modMap[rel.Modid] = modInfo
		}

		modList[idx] = &client.SimpleModList_Item{
			Modid:   rel.Modid,
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

func (kn *knossosServer) GetRemoteModInfo(ctx context.Context, req *client.ModInfoRequest) (*client.ModInfoResponse, error) {
	mod, err := storage.GetRemoteMod(ctx, req.Id)
	if err != nil {
		return nil, err
	}

	release, err := storage.GetRemoteModRelease(ctx, req.Id, req.Version)
	if err != nil {
		return nil, err
	}

	versions, err := storage.GetVersionsForRemoteMod(ctx, req.Id)
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
