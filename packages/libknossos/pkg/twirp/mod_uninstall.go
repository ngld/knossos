package twirp

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/ngld/knossos/packages/api/client"
	"github.com/ngld/knossos/packages/libknossos/pkg/api"
	"github.com/ngld/knossos/packages/libknossos/pkg/mods"
	"github.com/ngld/knossos/packages/libknossos/pkg/storage"
	"github.com/rotisserie/eris"
)

func (kn *knossosServer) UninstallModCheck(ctx context.Context, req *client.UninstallModCheckRequest) (*client.UninstallModCheckResponse, error) {
	versions, err := storage.LocalMods.GetVersionsForMod(ctx, req.Modid)
	if err != nil {
		return nil, eris.Wrapf(err, "failed to load versions for %s", req.Modid)
	}

	errors := make(map[string]string)
	for _, version := range versions {
		dependents, err := mods.GetModDependents(ctx, storage.LocalMods, req.Modid, version)
		if err != nil {
			return nil, eris.Wrapf(err, "failed to determine dependents for %s %s", req.Modid, version)
		}

		if len(dependents) > 0 {
			names := make([]string, len(dependents))
			for idx, pair := range dependents {
				mod, err := storage.LocalMods.GetMod(ctx, pair[0])
				if err != nil {
					return nil, eris.Wrapf(err, "failed to load mod info for %s", pair[0])
				}

				names[idx] = fmt.Sprintf("%s %s", mod.Title, pair[1])
			}

			if len(names) > 1 {
				lastIdx := len(names) - 1
				message := fmt.Sprintf("Can't uninstall this version because %s and %s depend on it.", strings.Join(names[:lastIdx], ", "), names[lastIdx])
				errors[version] = message
			} else {
				message := fmt.Sprintf("Can't uninstall this version because %s depends on it.", names[0])
				errors[version] = message
			}
		}
	}

	return &client.UninstallModCheckResponse{
		Versions: versions,
		Errors:   errors,
	}, nil
}

func (kn *knossosServer) UninstallMod(ctx context.Context, req *client.UninstallModRequest) (*client.SuccessResponse, error) {
	api.RunTask(ctx, req.Ref, func(ctx context.Context) error {
		for _, version := range req.Versions {
			rel, err := storage.LocalMods.GetModRelease(ctx, req.Modid, version)
			if err != nil {
				return eris.Wrapf(err, "failed to load release for %s %s", req.Modid, version)
			}

			folder, err := mods.GetModFolder(ctx, rel)
			if err != nil {
				return err
			}

			api.Log(ctx, api.LogInfo, "Deleting %s", folder)
			err = os.RemoveAll(folder)
			if err != nil {
				return eris.Wrapf(err, "failed to delete folder %s for mod %s %s", folder, req.Modid, version)
			}

			err = storage.DeleteLocalModRelease(ctx, rel)
			if err != nil {
				return eris.Wrapf(err, "failed to remove %s %s from mod database", req.Modid, version)
			}
		}

		api.Log(ctx, api.LogInfo, "Done")
		return nil
	})

	return &client.SuccessResponse{Success: true}, nil
}
