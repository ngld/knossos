module github.com/ngld/knossos/packages/updater

go 1.16

require (
	github.com/Masterminds/semver/v3 v3.1.1
	github.com/davecgh/go-spew v1.1.0
	github.com/go-gl/gl v0.0.0-20210501111010-69f74958bac0
	github.com/inkyblackness/imgui-go/v4 v4.2.0
	github.com/ngld/knossos/packages/libarchive v0.0.0-00010101000000-000000000000
	github.com/rotisserie/eris v0.5.0
	github.com/veandco/go-sdl2 v0.4.6
)

replace github.com/ngld/knossos/packages/libarchive => ../libarchive
