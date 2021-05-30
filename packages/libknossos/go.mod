module github.com/ngld/knossos/packages/libknossos

go 1.15

require (
	github.com/Masterminds/semver/v3 v3.1.1
	github.com/aidarkhanov/nanoid v1.0.8
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/gorilla/mux v1.8.0
	github.com/gorilla/websocket v1.4.2
	github.com/ngld/knossos/packages/api v0.0.0-00010101000000-000000000000
	github.com/ngld/knossos/packages/libarchive v0.0.0-00010101000000-000000000000
	github.com/rotisserie/eris v0.5.0
	github.com/rs/zerolog v1.22.0
	github.com/twitchtv/twirp v8.0.0+incompatible
	go.etcd.io/bbolt v1.3.5
	golang.org/x/net v0.0.0-20210521195947-fe42d452be8f
	golang.org/x/sys v0.0.0-20210521203332-0cec03c779c1
	google.golang.org/protobuf v1.26.0
)

replace github.com/ngld/knossos/packages/api => ../api

replace github.com/ngld/knossos/packages/libarchive => ../libarchive
