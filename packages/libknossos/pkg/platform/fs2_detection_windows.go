package platform

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	// we use the sqlite3 driver through sql.Open()
	_ "github.com/mattn/go-sqlite3"
	"github.com/ngld/knossos/packages/libknossos/pkg/api"
	"github.com/rotisserie/eris"
	"golang.org/x/sys/windows/registry"
)

func DetectSteamInstallation(ctx context.Context) (string, error) {
	api.Log(ctx, api.LogInfo, "Looking for Steam installation")
	api.SetProgress(ctx, 0, "Looking for Steam installation")

	key, err := registry.OpenKey(registry.CURRENT_USER, `SOFTWARE\Valve\Steam`, registry.QUERY_VALUE)
	if err != nil {
		return "", eris.Wrap(err, "failed to read steam path from registry")
	}
	defer key.Close()

	steamPath, _, err := key.GetStringValue("SteamPath")
	if err != nil {
		return "", eris.Wrap(err, "failed to read steam path from registry")
	}

	key.Close()

	steamLibraries := []string{filepath.Join(steamPath, "steamapps", "common")}
	content, err := os.ReadFile(filepath.Join(steamPath, "config", "libraryfolders.vdf"))
	if err != nil {
		api.Log(ctx, api.LogWarn, "Could not find libraryfolder.vdf, will only check the primary library! (%v)", err)
	} else {
		libRegex := regexp.MustCompile(`"path"\s+"([^"]+)"`)
		for _, match := range libRegex.FindAllStringSubmatch(string(content), -1) {
			libPath := strings.ReplaceAll(match[1], "\\\\", "\\")
			libPath = filepath.Join(libPath, "steamapps", "common")
			steamLibraries = append(steamLibraries, libPath)
		}
	}

	return checkLibraryFolders(ctx, steamLibraries)
}

type gogConfig struct {
	LibraryPath string `json:"libraryPath"`
	StoragePath string `json:"storagePath"`
}

func DetectGOGInstallation(ctx context.Context) (string, error) {
	api.Log(ctx, api.LogInfo, "Looking for GOG Galaxy installation")
	api.SetProgress(ctx, 0, "Looking for GOG Galaxy installation")

	/*galaxyPath, err := readRegKey(windows.HKEY_LOCAL_MACHINE, "SOFTWARE/WOW6432Node/GOG.com/GalaxyClient/paths", "client")
	if err != nil {
		return "", eris.Wrap(err, "failed to read galaxy path from registry")
	}*/

	progData := os.Getenv("ProgramData")
	configPath := filepath.Join(progData, "GOG.com", "Galaxy", "config.json")
	configBytes, err := os.ReadFile(configPath)
	if err != nil {
		return "", eris.Wrapf(err, "failed to read config %s", configPath)
	}

	var config gogConfig
	err = json.Unmarshal(configBytes, &config)
	if err != nil {
		return "", eris.Wrapf(err, "failed to parse config %s", configPath)
	}

	gogLibraries := []string{config.LibraryPath}
	api.Log(ctx, api.LogInfo, "Opening GOG Galaxy storage")

	db, err := sql.Open("sqlite3", "file:"+filepath.Join(config.StoragePath, "galaxy-2.0.db")+"?mode=ro")
	if err != nil {
		api.Log(ctx, api.LogWarn, "Failed to open GOG Galaxy storage (%v), will only check the primary library!", err)
	} else {
		rows, err := db.Query("SELECT installationPath FROM InstalledBaseProducts WHERE productId = 5")
		if err != nil {
			api.Log(ctx, api.LogWarn, "Failed to read from GOG Galaxy storage (%v), will only check the primary library!", err)
		} else {
			var path string
			for rows.Next() {
				err = rows.Scan(&path)
				if err != nil {
					api.Log(ctx, api.LogWarn, "Failed to read a row from GOG Galaxy storage (%v), will only check the primary library!", err)
				}

				gogLibraries = append(gogLibraries, filepath.Dir(path))
			}
		}

		db.Close()
	}

	return checkLibraryFolders(ctx, gogLibraries)
}

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
