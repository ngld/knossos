module github.com/ngld/knossos/packages/server

go 1.17

require (
	github.com/Masterminds/semver/v3 v3.1.1
	github.com/aidarkhanov/nanoid v1.0.8
	github.com/andskur/argon2-hashing v0.1.3
	github.com/cristalhq/aconfig v0.16.8
	github.com/cristalhq/aconfig/aconfigtoml v0.16.1
	github.com/davecgh/go-spew v1.1.1
	github.com/gorilla/mux v1.8.0
	github.com/jackc/pgconn v1.11.0
	github.com/jackc/pgtype v1.10.0
	github.com/jackc/pgx/v4 v4.15.0
	github.com/jordan-wright/email v4.0.1-0.20210109023952-943e75fe5223+incompatible
	github.com/ngld/knossos/packages/api v0.0.0-00010101000000-000000000000
	github.com/rotisserie/eris v0.5.2
	github.com/rs/zerolog v1.26.1
	github.com/ryanuber/go-glob v1.0.0 // indirect
	github.com/shaj13/go-guardian/v2 v2.11.5
	github.com/shaj13/libcache v1.0.0
	github.com/twitchtv/twirp v8.1.1+incompatible
	github.com/unrolled/secure v1.10.0
	github.com/zpatrick/rbac v0.0.0-20180829190353-d2c4f050cf28
	golang.org/x/crypto v0.0.0-20220321153916-2c7772ba3064 // indirect
	golang.org/x/sys v0.0.0-20220319134239-a9b59b0215f8 // indirect
	google.golang.org/protobuf v1.28.0
	gopkg.in/square/go-jose.v2 v2.6.0 // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
)

require (
	github.com/BurntSushi/toml v1.0.0 // indirect
	github.com/jackc/chunkreader/v2 v2.0.1 // indirect
	github.com/jackc/pgio v1.0.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgproto3/v2 v2.2.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20200714003250-2b9c44734f2b // indirect
	github.com/jackc/puddle v1.2.1 // indirect
	golang.org/x/text v0.3.7 // indirect
)

replace github.com/ngld/knossos/packages/api => ../api

replace github.com/ngld/knossos/packages/libknossos => ../libknossos

replace github.com/ngld/knossos/packages/libarchive => ../libarchive
