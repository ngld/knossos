module github.com/ngld/knossos/packages/build-tools

go 1.15

require (
	github.com/Microsoft/go-winio v0.5.0 // indirect
	github.com/VividCortex/ewma v1.2.0 // indirect
	github.com/aidarkhanov/nanoid v1.0.8
	github.com/andybalholm/brotli v1.0.3
	github.com/cheggaaa/pb/v3 v3.0.8
	github.com/containerd/containerd v1.5.4 // indirect
	github.com/cortesi/modd v0.0.0-20210323234521-b35eddab86cc
	github.com/cortesi/moddwatch v0.0.0-20210323234936-df014e95c743 // indirect
	github.com/docker/docker v20.10.7+incompatible // indirect
	github.com/fatih/color v1.12.0 // indirect
	github.com/golang/protobuf v1.5.2
	github.com/jackc/pgx/v4 v4.12.0 // indirect
	github.com/jschaf/pggen v0.0.0-20210622015421-8d43ddabaecf
	github.com/mattn/go-isatty v0.0.13 // indirect
	github.com/mattn/go-runewidth v0.0.13 // indirect
	github.com/mitchellh/colorstring v0.0.0-20190213212951-d06e56a500db
	github.com/peterbourgon/ff/v3 v3.1.0 // indirect
	github.com/rotisserie/eris v0.5.1
	github.com/rs/zerolog v1.23.0
	github.com/sirupsen/logrus v1.8.1 // indirect
	github.com/spf13/cobra v1.2.1
	github.com/twitchtv/twirp v8.1.0+incompatible
	github.com/ulikunitz/xz v0.5.10
	go.starlark.net v0.0.0-20210602144842-1cdb82c9e17a
	go.uber.org/atomic v1.9.0 // indirect
	go.uber.org/multierr v1.7.0 // indirect
	go.uber.org/zap v1.18.1 // indirect
	golang.org/x/crypto v0.0.0-20210711020723-a769d52b0f97 // indirect
	golang.org/x/net v0.0.0-20210716203947-853a461950ff // indirect
	golang.org/x/sys v0.0.0-20210630005230-0f9fa26af87c
	golang.org/x/term v0.0.0-20210615171337-6886f2dfbf5b // indirect
	google.golang.org/genproto v0.0.0-20210721163202-f1cecdd8b78a // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b
	mvdan.cc/sh/v3 v3.3.0
)

replace github.com/ngld/knossos/packages/libknossos => ../libknossos

replace github.com/ngld/knossos/packages/libarchive => ../libarchive

replace github.com/ngld/knossos/packages/api => ../api
