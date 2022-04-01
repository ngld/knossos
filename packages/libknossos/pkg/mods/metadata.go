package mods

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/ngld/knossos/packages/api/common"
	"github.com/ngld/knossos/packages/libknossos/pkg/storage"
	"github.com/rotisserie/eris"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func SaveLocalMod(ctx context.Context, mod *common.ModMeta) error {
	settings, err := storage.GetSettings(ctx)
	if err != nil {
		return eris.Wrap(err, "failed to read settings")
	}

	modData, err := json.MarshalIndent(mod, "", "  ")
	if err != nil {
		return eris.Wrapf(err, "failed to serialise mod metadata for %s", mod.Modid)
	}

	parent := mod.Parent
	if parent == "" || mod.Type == common.ModType_TOTAL_CONVERSION {
		parent = mod.Modid
	}

	modJSON := filepath.Join(settings.LibraryPath, parent, "knmod-"+mod.Modid+".json")
	err = os.WriteFile(modJSON, modData, 0o600)
	if err != nil {
		return eris.Wrapf(err, "failed to write %s for %s", modJSON, mod.Modid)
	}

	err = storage.SaveLocalMod(ctx, mod)
	if err != nil {
		return eris.Wrapf(err, "faile to write %s to local mod storage", mod.Modid)
	}

	return nil
}

func SaveLocalModRelease(ctx context.Context, rel *common.Release) error {
	modFolder, err := GetModFolder(ctx, rel)
	if err != nil {
		return eris.Wrapf(err, "failed to build folder path for %s %s", rel.Modid, rel.Version)
	}

	// TODO: This should be unnecessary since we've already installed the mod at this point.
	// If the folder doesn't exist, the mod installation must have failed.
	err = os.MkdirAll(modFolder, 0o700)
	if err != nil {
		return eris.Wrapf(err, "failed to create mod folder %s for %s %s", modFolder, rel.Modid, rel.Version)
	}

	releaseData, err := json.MarshalIndent(rel, "", "  ")
	if err != nil {
		return eris.Wrapf(err, "failed to serialise release %s %s", rel.Modid, rel.Version)
	}

	releaseJSON := filepath.Join(modFolder, "knrelease.json")
	err = os.WriteFile(releaseJSON, releaseData, 0o600)
	if err != nil {
		return eris.Wrapf(err, "failed to write %s for %s %s", releaseJSON, rel.Modid, rel.Version)
	}

	rel.JsonExportUpdated = timestamppb.Now()
	err = storage.SaveLocalModRelease(ctx, rel)
	if err != nil {
		return eris.Wrapf(err, "failed to save release %s (%s)", rel.Modid, rel.Version)
	}

	return nil
}
