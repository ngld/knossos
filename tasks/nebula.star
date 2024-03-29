load("helpers.star", "get_golangci_flags", "yarn")

neb_args = option("server_args", "", help = "The parameters to pass to Nebula in the server-run target")

db_network = option("db_network", "nebula", help = "The name of the Docker network to use for Nebula-related containers.")
db_container = option("db_container", "nebula-db", help = "The name of the Docker container for Nebula's managed database.")
db_port = option("db_port", "4142", help = "The port to expose Nebula's managed database on.")
db_user = option("db_user", "nebula", help = "The username to use for Nebula's managed database.")
db_pass = option("db_pass", "nebula", help = "The password to use for Nebula's managed database.")
db_name = option("db_name", "nebula", help = "The name of the database used by Nebula.")

def nebula_configure(binext):
    setenv("NEBULA_DATABASE", "postgres://%s:%s@localhost:%s/%s" % (db_user, db_pass, db_port, db_name))

    task(
        "database-setup",
        hidden = True,
        skip_if_exists = [".tools/db_setup"],
        cmds = [
            "docker network create '%s' || true" % db_network,
            "docker create --name '%s' --network '%s' -p '%s:5432' -e POSTGRES_USER='%s' -e POSTGRES_PASSWORD='%s' -e POSTGRES_DB='%s' postgres:alpine" % (db_container, db_network, db_port, db_user, db_pass, db_name),
            "touch .tools/db_setup",
        ],
    )

    task(
        "database-ready",
        hidden = True,
        deps = ["database-setup"],
        cmds = [
            "docker start '%s'" % db_container,
            "until docker exec '%s' pg_isready; do sleep 1; done" % db_container,
        ],
    )

    task(
        "database-migrate",
        desc = "Initializes and migrates the Nebula database (using Docker)",
        deps = ["database-ready"],
        inputs = ["db/migrations/*.sql"],
        outputs = [".tools/db_migrated"],
        cmds = [
            "docker run --rm --network '%s' -v \"$PWD/db/migrations:/flyway/sql\" flyway/flyway:latest-alpine -url='jdbc:postgresql://%s/%s?user=%s&password=%s' migrate" % (db_network, db_container, db_name, db_user, db_pass),
            "touch .tools/db_migrated",
        ],
    )

    importer_bin = resolve_path("//build/nebula/importer%s" % binext)
    task(
        "importer-build",
        hidden = True,
        deps = [],
        base = "packages/server",
        inputs = ["**/*.go"],
        outputs = [str(importer_bin)],
        cmds = [("go", "build", "-o", importer_bin, "./cmd/importer")],
    )

    task(
        "database-seed",
        desc = "Fills the database with the currently available mods from Nebula",
        deps = ["database-migrate", "importer-build"],
        base = "build/nebula",
        cmds = [
            "curl -Lo repo.json https://cf.fsnebula.org/storage/repo.json",
            "./importer",
        ],
    )

    task(
        "database-clean",
        desc = "Tears down the managed Nebula database",
        deps = ["build-tool"],
        ignore_exit = True,
        cmds = [
            "docker rm -f '%s'" % db_container,
            "docker network rm '%s'" % db_network,
            "rm -f .tools/db_*",
        ],
    )

    task(
        "server-lint",
        desc = "Lints server with golangci-lint",
        deps = ["fetch-deps", "proto-build", "database-migrate"],
        base = "packages/server",
        cmds = [
            "go generate ./pkg/db/queries.go",
            "golangci-lint run" + get_golangci_flags(),
        ],
    )

    neb_bin = resolve_path("build/nebula/nebula%s" % binext)

    task(
        "server-build",
        desc = "Compiles the Nebula server code",
        deps = ["proto-build", "database-migrate"],
        base = "packages/server",
        inputs = [
            "cmd/**/*.go",
            "pkg/**/*.go",
            "pkg/db/queries/*.sql",
        ],
        outputs = [str(neb_bin)],
        cmds = [
            "mkdir -p ../../build/nebula",
            "go generate -x ./pkg/db/queries.go",
            "go build -o '%s' ./cmd/server/main.go" % neb_bin,
        ],
    )

    task(
        "server-run",
        desc = "Launches Nebula",
        deps = ["server-build", "front-build", "database-migrate"],
        base = "packages/server",
        cmds = [(neb_bin, parse_shell_args(neb_args))],
    )

    task(
        "front-build",
        desc = "Builds the assets for Nebula's frontend",
        base = "packages/front",
        inputs = ["src/**/*.{ts,tsx,js,css}"],
        outputs = [
            "dist/prod/**/*.{html,css,js}",
        ],
        env = {
            "NODE_ENV": "production",
        },
        cmds = [yarn("webpack --env production --color --progress")],
    )

    task(
        "front-watch",
        desc = "Launches webpack-dev-server for Nebula's frontend",
        base = "packages/front",
        cmds = [yarn("webpack serve")],
    )

    if OS == "linux":
        container_bin_cmds = ["cp build/nebula/nebula build/nebula/nebula.linux"]
    else:
        container_bin_cmds = [
            "cd packages/server",
            ("GOOS=linux", "go", "build", "-o", "../../build/nebula/nebula.linux", "./cmd/server/main.go"),
            "cd ../..",
        ]

    task(
        "server-container",
        desc = "Builds the docker container for Nebula",
        deps = ["server-build", "front-build"],
        inputs = [
            str(neb_bin),
            "packages/server/Dockerfile",
        ],
        cmds = container_bin_cmds + [
            "cd build/nebula",
            "rm -rf templates front",
            "mkdir templates",
            "bash -c 'cp -r ../../packages/server/{Dockerfile,config.toml,templates} .'",
            "bash -c 'cp -r ../../packages/front/dist/prod front'",
            "docker build -tghcr.io/ngld/knossos/nebula:latest .",
        ]
    )
