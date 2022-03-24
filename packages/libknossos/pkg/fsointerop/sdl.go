package fsointerop

import (
	"context"

	"github.com/veandco/go-sdl2/sdl"
)

func GetPrefPath(ctx context.Context) string {
	// TODO: support portable mode

	// See https://github.com/scp-fs2open/fs2open.github.com/blob/18754fafc138591d2edfd0bc88ae02a6807091b7/code/osapi/osapi.cpp#L44
	return sdl.GetPrefPath("HardLightProductions", "FreeSpaceOpen")
}
