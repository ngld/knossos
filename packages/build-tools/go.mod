module github.com/ngld/knossos/packages/build-tools

go 1.15

require (
	github.com/Microsoft/go-winio v0.5.0 // indirect
	github.com/aidarkhanov/nanoid v1.0.8
	github.com/andybalholm/brotli v1.0.2
	github.com/cheggaaa/pb/v3 v3.0.8 // indirect
	github.com/containerd/containerd v1.5.2 // indirect
	github.com/cortesi/modd v0.0.0-20210323234521-b35eddab86cc
	github.com/cortesi/moddwatch v0.0.0-20210323234936-df014e95c743 // indirect
	github.com/docker/docker v20.10.6+incompatible // indirect
	github.com/golang/protobuf v1.5.2
	github.com/jackc/pgproto3/v2 v2.0.7 // indirect
	github.com/jschaf/pggen v0.0.0-20210517091311-cece7af82c5f
	github.com/mitchellh/colorstring v0.0.0-20190213212951-d06e56a500db
	github.com/rotisserie/eris v0.5.0
	github.com/rs/zerolog v1.22.0
	github.com/schollz/progressbar/v3 v3.8.1
	github.com/sirupsen/logrus v1.8.1 // indirect
	github.com/spf13/cobra v1.1.3
	github.com/twitchtv/twirp v8.0.0+incompatible
	github.com/ulikunitz/xz v0.5.10
	go.starlark.net v0.0.0-20210511153848-cca21e7857d4
	go.uber.org/multierr v1.7.0 // indirect
	go.uber.org/zap v1.16.0 // indirect
	golang.org/x/crypto v0.0.0-20210513164829-c07d793c2f9a // indirect
	golang.org/x/mod v0.4.2 // indirect
	golang.org/x/net v0.0.0-20210521195947-fe42d452be8f // indirect
	golang.org/x/sys v0.0.0-20210521203332-0cec03c779c1
	google.golang.org/genproto v0.0.0-20210521181308-5ccab8a35a9a // indirect
	google.golang.org/grpc v1.38.0 // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b
	mvdan.cc/sh/v3 v3.3.0
)

replace github.com/ngld/knossos/packages/libknossos => ../libknossos

replace github.com/ngld/knossos/packages/libarchive => ../libarchive

replace github.com/ngld/knossos/packages/api => ../api
