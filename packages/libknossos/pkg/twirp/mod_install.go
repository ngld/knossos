package twirp

import (
	"context"

	"github.com/ngld/knossos/packages/api/client"
	"github.com/ngld/knossos/packages/libknossos/pkg/api"
	"github.com/ngld/knossos/packages/libknossos/pkg/mods"
	"github.com/ngld/knossos/packages/libknossos/pkg/storage"
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

func (kn *knossosServer) InstallMod(ctx context.Context, req *client.InstallModRequest) (*client.SuccessResponse, error) {
	api.RunTask(ctx, req.Ref, func(ctx context.Context) error {
		err := mods.InstallMod(ctx, req)
		api.Log(ctx, api.LogInfo, "Done")

		return err
	})
	return &client.SuccessResponse{Success: true}, nil
}
