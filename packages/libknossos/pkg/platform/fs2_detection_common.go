package platform

import (
	"context"
	"os"
	"path/filepath"

	"github.com/ngld/knossos/packages/libknossos/pkg/api"
	"github.com/rotisserie/eris"
)

func checkLibraryFolders(ctx context.Context, folders []string) (string, error) {
	api.Log(ctx, api.LogInfo, "Looking for game folder in %d libraries", len(folders))

	for _, folder := range folders {
		for _, variant := range []string{"Freespace 2", "Freespace2", "freespace2", "freespace 2"} {
			gameFolder := filepath.Join(folder, variant)
			api.Log(ctx, api.LogDebug, "Checking %s", gameFolder)
			_, err := os.Stat(filepath.Join(gameFolder, "root_fs2.vp"))
			if err == nil {
				return gameFolder, nil
			}
		}
	}

	return "", eris.New("no game folder found")
}
