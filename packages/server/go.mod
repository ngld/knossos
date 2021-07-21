module github.com/ngld/knossos/packages/server

go 1.15

require (
	github.com/Masterminds/semver/v3 v3.1.1
	github.com/aidarkhanov/nanoid v1.0.8
	github.com/andskur/argon2-hashing v0.1.3
	github.com/cristalhq/aconfig v0.16.2
	github.com/cristalhq/aconfig/aconfigtoml v0.16.1
	github.com/davecgh/go-spew v1.1.1
	github.com/gorilla/mux v1.8.0
	github.com/jackc/pgconn v1.9.0
	github.com/jackc/pgtype v1.8.0
	github.com/jackc/pgx/v4 v4.12.0
	github.com/jordan-wright/email v4.0.1-0.20210109023952-943e75fe5223+incompatible
	github.com/kr/text v0.2.0 // indirect
	github.com/ngld/knossos/packages/api v0.0.0-20210718163256-09871a18e506
	github.com/rotisserie/eris v0.5.1
	github.com/rs/zerolog v1.23.0
	github.com/ryanuber/go-glob v1.0.0 // indirect
	github.com/shaj13/go-guardian/v2 v2.11.3
	github.com/shaj13/libcache v1.0.0
	github.com/twitchtv/twirp v8.1.0+incompatible
	github.com/unrolled/secure v1.0.9
	github.com/zpatrick/rbac v0.0.0-20180829190353-d2c4f050cf28
	golang.org/x/crypto v0.0.0-20210711020723-a769d52b0f97 // indirect
	golang.org/x/sys v0.0.0-20210630005230-0f9fa26af87c // indirect
	google.golang.org/protobuf v1.27.1
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gopkg.in/square/go-jose.v2 v2.6.0 // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
)

replace github.com/ngld/knossos/packages/api => ../api

replace github.com/ngld/knossos/packages/libknossos => ../libknossos

replace github.com/ngld/knossos/packages/libarchive => ../libarchive
