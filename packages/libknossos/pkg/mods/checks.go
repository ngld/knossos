package mods

import (
	"context"
	"runtime"

	"github.com/ngld/knossos/packages/api/common"
	"github.com/ngld/knossos/packages/libknossos/pkg/api"
	"github.com/ngld/knossos/packages/libknossos/pkg/helpers"
	"golang.org/x/sys/cpu"
)

func FilterUnsupportedPackages(ctx context.Context, pkgs []*common.Package) []*common.Package {
	result := make([]*common.Package, 0, len(pkgs))

	for _, pkg := range pkgs {
		skip := false

		for _, feat := range pkg.CpuSpec.GetRequiredFeatures() {
			switch feat {
			case "":
				// this shouldn't happen but is harmless; just ignore it
			case "windows", "darwin", "linux":
				skip = runtime.GOOS != feat
			case "macosx":
				skip = runtime.GOOS != "darwin"
			case "x86_64":
				skip = !helpers.SupportsX64()
			case "avx":
				skip = !cpu.X86.HasAVX
			case "avx2":
				skip = !cpu.X86.HasAVX2
			default:
				api.Log(ctx, api.LogDebug, "Skipping package %s because feature %s is unknown", pkg.Name, feat)
				skip = true
			}

			if skip {
				break
			}
		}

		if !skip {
			result = append(result, pkg)
		}
	}

	return result
}
