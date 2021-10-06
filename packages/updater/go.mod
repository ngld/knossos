module github.com/ngld/knossos/packages/updater

go 1.17

require (
	github.com/Masterminds/semver/v3 v3.1.1
	github.com/go-gl/gl v0.0.0-20210905235341-f7a045908259
	github.com/inkyblackness/imgui-go/v4 v4.3.0
	github.com/ngld/knossos/packages/libarchive v0.0.0-20211005231007-d4686ca19d5d
	github.com/rotisserie/eris v0.5.1
	github.com/veandco/go-sdl2 v0.4.10
)

replace github.com/ngld/knossos/packages/libarchive => ../libarchive
