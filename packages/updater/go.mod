module github.com/ngld/knossos/packages/updater

go 1.19

require (
	github.com/Masterminds/semver/v3 v3.2.0
	github.com/go-gl/gl v0.0.0-20211210172815-726fda9656d6
	github.com/inkyblackness/imgui-go/v4 v4.7.0
	github.com/ngld/knossos/packages/libarchive v0.0.0-20220412214947-82b21dfb166e
	github.com/rotisserie/eris v0.5.4
)

require github.com/stretchr/testify v1.8.0 // indirect

replace github.com/ngld/knossos/packages/libarchive => ../libarchive

