//go:build tools

package main

import (
	_ "github.com/jschaf/pggen/cmd/pggen"
	_ "github.com/twitchtv/twirp/protoc-gen-twirp"
	_ "golang.org/x/tools/cmd/stringer"
	_ "google.golang.org/protobuf/cmd/protoc-gen-go"
)
