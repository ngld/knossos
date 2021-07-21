module github.com/ngld/knossos/packages/libknossos

go 1.15

require (
	github.com/Masterminds/semver/v3 v3.1.1
	github.com/aidarkhanov/nanoid v1.0.8
	github.com/davecgh/go-spew v1.1.1
	github.com/gorilla/mux v1.8.0
	github.com/gorilla/websocket v1.4.2
	github.com/ngld/knossos/packages/api v0.0.0-20210718163256-09871a18e506
	github.com/ngld/knossos/packages/libarchive v0.0.0-20210718163256-09871a18e506
	github.com/rotisserie/eris v0.5.1
	github.com/rs/zerolog v1.23.0
	github.com/twitchtv/twirp v8.1.0+incompatible
	go.etcd.io/bbolt v1.3.6
	golang.org/x/net v0.0.0-20210716203947-853a461950ff
	golang.org/x/sys v0.0.0-20210630005230-0f9fa26af87c
	google.golang.org/protobuf v1.27.1
)

replace github.com/ngld/knossos/packages/api => ../api

replace github.com/ngld/knossos/packages/libarchive => ../libarchive
