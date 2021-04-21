module github.com/ngld/knossos/packages/libknossos

go 1.15

require (
	github.com/Masterminds/semver/v3 v3.1.1
	github.com/aidarkhanov/nanoid v1.0.8
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/gorilla/mux v1.8.0 // indirect
	github.com/gorilla/websocket v1.4.2 // indirect
	github.com/ngld/knossos/packages/api v0.0.0-00010101000000-000000000000
	github.com/ngld/knossos/packages/libarchive v0.0.0-00010101000000-000000000000
	github.com/rotisserie/eris v0.5.0
	github.com/rs/zerolog v1.21.0 // indirect
	github.com/twitchtv/twirp v7.1.1+incompatible // indirect
	go.etcd.io/bbolt v1.3.5
	golang.org/x/sys v0.0.0-20210403161142-5e06dd20ab57
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	google.golang.org/protobuf v1.26.0
)

replace github.com/ngld/knossos/packages/api => ../api

replace github.com/ngld/knossos/packages/libarchive => ../libarchive
