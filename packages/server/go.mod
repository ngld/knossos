module github.com/ngld/knossos/packages/server

go 1.19

require (
	github.com/Masterminds/semver/v3 v3.2.0
	github.com/aidarkhanov/nanoid v1.0.8
	github.com/andskur/argon2-hashing v0.1.3
	github.com/cristalhq/aconfig v0.18.3
	github.com/cristalhq/aconfig/aconfigtoml v0.17.1
	github.com/davecgh/go-spew v1.1.1
	github.com/gorilla/mux v1.8.0
	github.com/jackc/pgconn v1.13.0
	github.com/jackc/pgtype v1.13.0
	github.com/jackc/pgx/v4 v4.17.2
	github.com/jordan-wright/email v4.0.1-0.20210109023952-943e75fe5223+incompatible
	github.com/ngld/knossos/packages/api v0.0.0-20220412214947-82b21dfb166e
	github.com/rotisserie/eris v0.5.4
	github.com/rs/zerolog v1.28.0
	github.com/ryanuber/go-glob v1.0.0 // indirect
	github.com/shaj13/go-guardian/v2 v2.11.5
	github.com/shaj13/libcache v1.0.5
	github.com/twitchtv/twirp v8.1.3+incompatible
	github.com/unrolled/secure v1.13.0
	github.com/zpatrick/rbac v0.0.0-20180829190353-d2c4f050cf28
	golang.org/x/crypto v0.5.0 // indirect
	golang.org/x/sys v0.4.0 // indirect
	google.golang.org/protobuf v1.28.1
	gopkg.in/square/go-jose.v2 v2.6.0 // indirect
)

require (
	github.com/minio/sha256-simd v1.0.0
	github.com/ngld/knossos/packages/libarchive v0.0.0-20220412214947-82b21dfb166e
)

require (
	github.com/BurntSushi/toml v1.2.1 // indirect
	github.com/jackc/chunkreader/v2 v2.0.1 // indirect
	github.com/jackc/pgio v1.0.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgproto3/v2 v2.3.1 // indirect
	github.com/jackc/pgservicefile v0.0.0-20221227161230-091c0ba34f0a // indirect
	github.com/jackc/puddle v1.3.0 // indirect
	github.com/klauspost/cpuid/v2 v2.2.3 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.17 // indirect
	golang.org/x/text v0.6.0 // indirect
)

replace github.com/ngld/knossos/packages/api => ../api

replace github.com/ngld/knossos/packages/libknossos => ../libknossos

replace github.com/ngld/knossos/packages/libarchive => ../libarchive
