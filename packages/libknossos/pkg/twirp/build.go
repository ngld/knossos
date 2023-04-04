package twirp

import (
	"context"

	"github.com/ngld/knossos/packages/api/client"
	"github.com/ngld/knossos/packages/libknossos/pkg/api"
	"github.com/ngld/knossos/packages/libknossos/pkg/storage"
)

func (kn *knossosServer) GetSimpleModList(ctx context.Context, req *client.NullMessage) (*client.SimpleModListResponse, error) {
	releases, err := storage.LocalMods.GetAllReleases(ctx)
	if err != nil {
		return nil, err
	}

	titles := make(map[string]string)

	infos := make([]*client.SimpleModListResponse_ModInfo, len(releases))
	for idx, rel := range releases {
		title, ok := titles[rel.Modid]
		if !ok {
			mod, err := storage.LocalMods.GetMod(ctx, rel.Modid)
			if err != nil {
				api.Log(ctx, api.LogError, "Failed to retrieve title for mod %s", rel.Modid)
			} else {
				title = mod.Title
				titles[rel.Modid] = title
			}
		}

		infos[idx] = &client.SimpleModListResponse_ModInfo{
			Modid:   rel.Modid,
			Version: rel.Version,
			Title:   title,
		}
	}

	return &client.SimpleModListResponse{Mods: infos}, nil
}

func (kn *knossosServer) GetBuildModRelInfo(ctx context.Context, req *client.ModInfoRequest) (*client.BuildModRelInfoResponse, error) {
	rel, err := storage.LocalMods.GetModRelease(ctx, req.Id, req.Version)
	if err != nil {
		return nil, err
	}

	return &client.BuildModRelInfoResponse{Packages: rel.Packages}, nil
}
