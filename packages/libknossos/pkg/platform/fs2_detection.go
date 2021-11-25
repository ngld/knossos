//go:build !windows && !linux

package platform

import (
	"context"

	"github.com/ngld/knossos/packages/libknossos/pkg/api"
)

func DetectSteamInstallation(ctx context.Context) (string, error) {
	api.Log(ctx, api.LogError, "Steam detection hasn't been implemented for this platform, yet.")
	return "", nil
}

func DetectGOGInstallation(ctx context.Context) (string, error) {
	api.Log(ctx, api.LogError, "GOG detection hasn't been implemented for this platform, yet.")
	return "", nil
}
