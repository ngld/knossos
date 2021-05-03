module github.com/ngld/knossos/packages/libknossos

go 1.15

require (
	github.com/Masterminds/semver/v3 v3.1.1
	github.com/aidarkhanov/nanoid v1.0.8
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/gorilla/mux v1.8.0
	github.com/gorilla/websocket v1.4.2
	github.com/ngld/knossos/packages/api v0.0.0-00010101000000-000000000000
	github.com/ngld/knossos/packages/libarchive v0.0.0-00010101000000-000000000000
	github.com/rotisserie/eris v0.5.0
	github.com/rs/zerolog v1.21.0
	github.com/twitchtv/twirp v7.2.0+incompatible // indirect
	go.etcd.io/bbolt v1.3.5
	golang.org/x/sys v0.0.0-20210503173754-0981d6026fa6
	google.golang.org/protobuf v1.26.0
)

replace github.com/ngld/knossos/packages/api => ../api

replace github.com/ngld/knossos/packages/libarchive => ../libarchive
