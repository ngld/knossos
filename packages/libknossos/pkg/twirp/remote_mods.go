package twirp

import (
	"context"

	"github.com/ngld/knossos/packages/api/client"
	"github.com/ngld/knossos/packages/api/common"
	"github.com/ngld/knossos/packages/libknossos/pkg/api"
	"github.com/ngld/knossos/packages/libknossos/pkg/mods"
	"github.com/ngld/knossos/packages/libknossos/pkg/storage"
)

func (kn *knossosServer) SyncRemoteMods(ctx context.Context, task *client.TaskRequest) (*client.SuccessResponse, error) {
	api.RunTask(ctx, task.Ref, mods.UpdateRemoteModIndex)
	return &client.SuccessResponse{Success: true}, nil
}

func (kn *knossosServer) GetRemoteMods(ctx context.Context, _ *client.NullMessage) (*client.SimpleModList, error) {
	return buildModList(ctx, storage.RemoteMods)
}

func (kn *knossosServer) GetRemoteModInfo(ctx context.Context, req *client.ModInfoRequest) (*client.ModInfoResponse, error) {
	mod, err := storage.RemoteMods.GetMod(ctx, req.Id)
	if err != nil {
		return nil, err
	}

	release, err := storage.RemoteMods.GetModRelease(ctx, req.Id, req.Version)
	if err != nil {
		return nil, err
	}

	versions, err := storage.RemoteMods.GetVersionsForMod(ctx, req.Id)
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
