module github.com/ngld/knossos/packages/libknossos

go 1.19

require (
	github.com/Masterminds/semver/v3 v3.2.0
	github.com/aidarkhanov/nanoid v1.0.8
	github.com/davecgh/go-spew v1.1.1
	github.com/gorilla/mux v1.8.0
	github.com/gorilla/websocket v1.5.0
	github.com/mattn/go-sqlite3 v1.14.16
	github.com/ngld/knossos/packages/api v0.0.0-00010101000000-000000000000
	github.com/ngld/knossos/packages/libarchive v0.0.0-00010101000000-000000000000
	github.com/ngld/knossos/packages/libopenal v0.0.0-00010101000000-000000000000
	github.com/rotisserie/eris v0.5.4
	github.com/rs/zerolog v1.29.0
	github.com/twitchtv/twirp v8.1.3+incompatible
	github.com/veandco/go-sdl2 v0.4.33
	go.etcd.io/bbolt v1.3.7
	golang.org/x/net v0.8.0
	golang.org/x/sys v0.6.0
	google.golang.org/protobuf v1.30.0
)

require (
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.18 // indirect
)

replace github.com/ngld/knossos/packages/api => ../api

replace github.com/ngld/knossos/packages/libarchive => ../libarchive

replace github.com/ngld/knossos/packages/libinnoextract => ../libinnoextract

replace github.com/ngld/knossos/packages/libopenal => ../libopenal
