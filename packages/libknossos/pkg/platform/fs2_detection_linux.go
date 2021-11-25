package platform

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ngld/knossos/packages/libknossos/pkg/api"
)

func DetectSteamInstallation(ctx context.Context) (string, error) {
	api.Log(ctx, api.LogInfo, "Looking for Steam installation")
	api.SetProgress(ctx, 0, "Looking for Steam installation")

	folderConfig := filepath.Join(os.Getenv("HOME"), ".steam", "steam", "config", "libraryfolders.vdf")
	steamLibraries := []string{}
	content, err := os.ReadFile(folderConfig)

	if err != nil {
		api.Log(ctx, api.LogWarn, "Could not read %s, will only check the primary library! (%v)", folderConfig, err)
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

func DetectGOGInstallation(ctx context.Context) (string, error) {
	api.Log(ctx, api.LogError, "GOG detection hasn't been implemented for this platform, yet.")
	return "", nil
}
