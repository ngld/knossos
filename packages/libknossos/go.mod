module github.com/ngld/knossos/packages/libknossos

go 1.17

require (
	github.com/Masterminds/semver/v3 v3.1.1
	github.com/aidarkhanov/nanoid v1.0.8
	github.com/davecgh/go-spew v1.1.1
	github.com/gorilla/mux v1.8.0
	github.com/gorilla/websocket v1.5.0
	github.com/mattn/go-sqlite3 v1.14.12
	github.com/ngld/knossos/packages/api v0.0.0-00010101000000-000000000000
	github.com/ngld/knossos/packages/libarchive v0.0.0-00010101000000-000000000000
	github.com/ngld/knossos/packages/libinnoextract v0.0.0-00010101000000-000000000000
	github.com/ngld/knossos/packages/libopenal v0.0.0-00010101000000-000000000000
	github.com/rotisserie/eris v0.5.2
	github.com/rs/zerolog v1.26.1
	github.com/twitchtv/twirp v8.1.1+incompatible
	github.com/veandco/go-sdl2 v0.4.18
	go.etcd.io/bbolt v1.3.6
	golang.org/x/net v0.0.0-20220225172249-27dd8689420f
	golang.org/x/sys v0.0.0-20220319134239-a9b59b0215f8
	google.golang.org/protobuf v1.28.0
)

replace github.com/ngld/knossos/packages/api => ../api

replace github.com/ngld/knossos/packages/libarchive => ../libarchive

replace github.com/ngld/knossos/packages/libinnoextract => ../libinnoextract

replace github.com/ngld/knossos/packages/libopenal => ../libopenal
