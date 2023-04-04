package mods

import (
	"context"
	"path/filepath"

	"github.com/ngld/knossos/packages/api/common"
	"github.com/ngld/knossos/packages/libknossos/pkg/storage"
	"github.com/rotisserie/eris"
)

func GetModFolder(ctx context.Context, rel *common.Release) (string, error) {
	folder := rel.Folder

	if !filepath.IsAbs(folder) {
		settings, err := storage.GetSettings(ctx)
		if err != nil {
			return "", eris.Wrap(err, "failed to load Knossos settings")
		}

		mod, err := storage.LocalMods.GetMod(ctx, rel.Modid)
		if err != nil {
			return "", eris.Wrapf(err, "failed to load mod %s", rel.Modid)
		}

		switch {
		case mod.Type == common.ModType_ENGINE || mod.Type == common.ModType_TOOL:
			folder = filepath.Join(settings.LibraryPath, "bin", folder)
		default:
			folder = filepath.Join(settings.LibraryPath, folder)
		}
	}

	return folder, nil
}
