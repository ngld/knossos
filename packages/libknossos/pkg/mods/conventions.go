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

		if mod.Type == common.ModType_ENGINE || mod.Type == common.ModType_TOOL {
			folder = filepath.Join(settings.LibraryPath, "bin", folder)
		} else if mod.Type == common.ModType_TOTAL_CONVERSION {
			folder = filepath.Join(settings.LibraryPath, mod.Modid, folder)
		} else if mod.Parent == "" {
			return "", eris.Errorf("mod %s is neither a TC nor an engine but doesn't have a parent", rel.Modid)
		} else {
			folder = filepath.Join(settings.LibraryPath, mod.Parent, folder)
		}
	}

	return folder, nil
}
