module github.com/ngld/knossos/packages/server

go 1.15

require (
	github.com/Masterminds/semver/v3 v3.1.1
	github.com/aidarkhanov/nanoid v1.0.8
	github.com/andskur/argon2-hashing v0.1.3
	github.com/cristalhq/aconfig v0.13.4
	github.com/cristalhq/aconfig/aconfigtoml v0.12.0
	github.com/gorilla/mux v1.8.0
	github.com/jackc/pgconn v1.8.1
	github.com/jackc/pgproto3/v2 v2.0.7 // indirect
	github.com/jackc/pgtype v1.7.0
	github.com/jackc/pgx/v4 v4.11.0
	github.com/jordan-wright/email v4.0.1-0.20210109023952-943e75fe5223+incompatible
	github.com/kr/text v0.2.0 // indirect
	github.com/lib/pq v1.10.1 // indirect
	github.com/ngld/knossos/packages/api v0.0.0-00010101000000-000000000000
	github.com/ngld/knossos/packages/libknossos v0.0.0-00010101000000-000000000000
	github.com/rotisserie/eris v0.5.0
	github.com/rs/zerolog v1.21.0
	github.com/ryanuber/go-glob v1.0.0 // indirect
	github.com/shaj13/go-guardian/v2 v2.11.3
	github.com/shaj13/libcache v1.0.0
	github.com/twitchtv/twirp v7.2.0+incompatible
	github.com/unrolled/secure v1.0.8
	github.com/zpatrick/rbac v0.0.0-20180829190353-d2c4f050cf28
	golang.org/x/crypto v0.0.0-20210421170649-83a5a9bb288b // indirect
	golang.org/x/text v0.3.6 // indirect
	google.golang.org/protobuf v1.26.0
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
)

replace github.com/ngld/knossos/packages/api => ../api

replace github.com/ngld/knossos/packages/libknossos => ../libknossos

replace github.com/ngld/knossos/packages/libarchive => ../libarchive
