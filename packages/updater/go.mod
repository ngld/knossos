module github.com/ngld/knossos/packages/updater

go 1.17

require (
	github.com/Masterminds/semver/v3 v3.1.1
	github.com/go-gl/gl v0.0.0-20211210172815-726fda9656d6
	github.com/inkyblackness/imgui-go/v4 v4.4.0
	github.com/ngld/knossos/packages/libarchive v0.0.0-00010101000000-000000000000
	github.com/rotisserie/eris v0.5.2
	github.com/veandco/go-sdl2 v0.5.0-alpha.3
)

replace github.com/ngld/knossos/packages/libarchive => ../libarchive
