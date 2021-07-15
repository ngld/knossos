"""
Build tasks for the Knossos / Nebula project

This file is written in Starlark a subset of Python.
A full specification can be found here: https://github.com/google/starlark-go/blob/master/doc/spec.md

load() is the equivalent of Python's import
  i.e. load("something.star", "a", b="c") is equivalent to "from something import a, c as b").

Quick API reference:
  option(name, default): returns a command line option
  resolve_path(some, path, parts): joins the given path elements and resolves any special syntax (like "//")
  prepend_path(path): prepends the given path to the OS $PATH variable (only affects tasks launched from this script)
  getenv(name): returns the value of the given environment var
  setenv(name, value): overrides the given environment variable for any process launched from this script
  task(name, ...): define a new target

Path help:
  resolve_path(...)
    any path passed to this function is assumed to be relative to this script
    // is an alias for the project root which currently is just the directory that contains this script

  task(...):
    any option that contains paths is automatically processed by resolve_path() and thus follows the same rules

Task help:
  desc: a description; only displayed in the CLI help
  deps: a list of targets which should be run (if necessary) before this task
  base: working directory; all other paths specified in this task are relative to this path
  skip_if_exists: a list of files; if this task is called as a dependency of another task and at least one of the
    listed files exists, this task is skipped
  inputs: a list of files
  outputs: a list of files; if this task is called as a dependency of another task and all outputs exist and are newer
    than the input files, this task is skipped
  ignore_exit: boolean (default False), disregard exit status of cmds when True
  cmds: a list of commands to execute
    the following types are allowed as list items:
      string: Will be interpreted as a shell script. Bash syntax is supported (even on Windows)
      tuple: a list of command arguments the first item which does not contain a "=" is the command that should be run
        all items preceeding it are env vars which should be set for this sub process
        all items after the command are arguments which are passed as-is (no globs, shell expansion, etc)
      task: a reference to another task which will be called at exactly this point
"""

load("tasks/options.star", "build", "generator_opt", "msys2_path")
load("tasks/helpers.star", "protoc", "yarn")
load("tasks/nebula.star", "nebula_configure")
load("tasks/knossos.star", "get_libarchive_flags", "knossos_configure")

def configure():
    generator = generator_opt

    if build not in ("Debug", "Release"):
        error("Invalid build mode %s passed. Only Debug or Release are valid." % build)

    setenv("NODE_OPTIONS", '-r "%s"' % to_slashes(str(resolve_path("//.pnp.js"))))

    if OS == "windows":
        libext = ".dll"
        binext = ".exe"

        prepend_path(resolve_path(msys2_path, "mingw64/bin"))
        setenv("GCCPATH", str(resolve_path(msys2_path, "mingw64/bin/gcc.exe")))

        prepend_path("third_party/ninja")

        compiler = getenv("CXX")
        if compiler == "":
            # user didn't specify a compiler, let's make sure we have a valid compiler
            if getenv("LIB") == "":
                # VC vars aren't set, run vcvarsall.bat to fix that
                info("Calling vcvarsall.bat")
                load_vcvars("amd64")

            # TODO Figure out how to properly disable /MP in the client package
            #if execute("clang-cl /?") != False:
            #    info("Using auto-detected clang-cl")

            #    setenv("CC", "clang-cl")
            #    setenv("CXX", "clang-cl")
            if execute("cl /?") != False:
                info("Using auto-detected cl")

                setenv("CC", "cl")
                setenv("CXX", "cl")
            else:
                error("No usable compiler found. CMake will fall back to gcc and fail under these circumstances")

        if generator == "":
            # ninja is always available because we download it in our fetch-deps step
            generator = "Ninja"

        info("Using MSYS2 installation at %s." % resolve_path(msys2_path))

    elif OS == "darwin":
        libext = ".dylib"
        binext = ""

        if generator == "":
            if execute("ninja -h") != False:
                generator = "Ninja"
            else:
                generator = "Unix Makefiles"

        if isdir("/usr/local/opt/ccache/libexec"):
            prepend_path("/usr/local/opt/ccache/libexec")
            info("Using ccache at /usr/local/opt/ccache/libexec")
    else:
        libext = ".so"
        binext = ""

        setenv("CFLAGS", "-fPIC " + getenv("CFLAGS"))

        if generator == "":
            if execute("ninja -h") != False:
                generator = "Ninja"
            elif execute("ninja-build -h") != False:
                # TODO fix the hard references to ninja's name
                warn("Expected ninja to be available as ninja, not ninja-build, falling back to make")
                generator = "Unix Makefiles"
            else:
                generator = "Unix Makefiles"

    prepend_path("third_party/go/bin")
    prepend_path("third_party/protoc-dist")
    prepend_path("third_party/nodejs" if OS == "windows" else "third_party/nodejs/bin")
    prepend_path("third_party/golangci")
    prepend_path(".tools")

    tool_bin = resolve_path(".tools/tool%s" % binext)

    build_tool_extra_cmds = []
    if OS == "windows":
        build_tool_extra_cmds = [
            "mv '%s' \"%s.old.$$\"" % (tool_bin, tool_bin),
        ]

    task(
        "build-tool",
        desc = "Build our build tool",
        base = "packages/build-tools",
        inputs = ["**/*.go"],
        outputs = [str(tool_bin)],
        cmds = build_tool_extra_cmds + [
            "go build -o '%s'" % tool_bin,
        ],
    )

    extra_tools = []
    task(
        "install-tools",
        desc = "Installs necessary go tools in the workspace (task, pggen, protoc plugins, ...)",
        deps = ["build-tool"],
        inputs = [
            "packages/build-tools/tools.go",
            "packages/build-tools/go.mod",
            "packages/build-tools/ccache-helper/main.go",
            "packages/build-tools/protoc-ts-helper/main.go",
        ],
        outputs = [".tools/%s%s" % (name, binext) for name in ("modd", "pggen", "protoc-gen-go", "protoc-gen-twirp", "protoc-gen-ts")],
        cmds = [
            "tool install-tools",
            "cd packages/build-tools",
            "go build -o ../../.tools/protoc-gen-ts%s ./protoc-ts-helper" % binext,
            "cd ../..",
        ] + extra_tools,
    )

    js_deps = task(
        "yarn-install",
        hidden = True,
        inputs = [
            "package.json",
            "yarn.lock",
        ],
        outputs = [
            ".yarn/cache/*.zip",
            ".pnp.js",
        ],
        env = {
            # The .pnp.js file doesn't exist, yet, so forcing Node.js to load it will cause yarn install to fail.
            "NODE_OPTIONS": "",
        },
        cmds = [
            yarn("install"),
            "touch .pnp.js",
        ],
    )

    task(
        "fetch-deps",
        desc = "Automatically downloads dependencies not covered by install-tools",
        deps = ["build-tool"],
        cmds = [
            "tool fetch-deps",
            js_deps,
        ],
    )

    task(
        "update-deps",
        desc = "Update the checksums listed in DEPS.yml (only use this if you manually changed that file)",
        deps = ["build-tool"],
        cmds = ["tool fetch-deps -u"],
    )

    task(
        "check-deps",
        desc = "Checks the dependencies listed in DEPS.yml for updates",
        deps = ["build-tool"],
        cmds = ["tool check-deps"],
    )

    task(
        "proto-build",
        desc = "Generates TS and Go bindings from the .proto API definitions",
        deps = ["fetch-deps", "install-tools"],
        base = "packages/api",
        inputs = ["definitions/*.proto"],
        outputs = [
            "api/{api,client,common}/*.{ts,go}",
            "client/**/*.go",
        ],
        cmds = [
            protoc("google/protobuf/timestamp.proto", ts = "api"),
            protoc("mod.proto", go = "common", ts = "api"),
            protoc("modsync.proto", go = "common", ts = "api"),
            protoc("client.proto", go = "client", twirp = "twirp", ts = "api"),
            protoc("service.proto", go = "api", twirp = "twirp", ts = "api"),
            # twirp doesn't support go.mod paths so we have to move the generated files to the correct location
            "mv twirp/github.com/ngld/knossos/packages/api/api/*.go api",
            "mv twirp/github.com/ngld/knossos/packages/api/client/*.go client",
            "rm -r twirp/github.com",
        ],
    )

    task(
        "js-lint",
        desc = "Check JS code for common issues",
        deps = ["fetch-deps", "proto-build"],
        cmds = [yarn("lint")],
    )

    nebula_configure(binext)
    knossos_configure(binext, libext, generator)

    updater_ldflags = ""
    updater_goldflags = "-s -w"
    if OS == "windows":
        updater_goldflags += " -H windowsgui -extldflags -static"

    task(
        "updater-build",
        desc = "Builds the Knossos updater",
        deps = ["libarchive-build"],
        env = {
            "CC": "gcc",
            "CXX": "g++",
            "CGO_LDFLAGS": get_libarchive_flags() + updater_ldflags,
        },
        cmds = [
            "mkdir -p build/updater",
            "cd packages/updater",
            "go build -tags static -ldflags '%s' -o ../../build/updater/updater%s" % (updater_goldflags, binext),
        ],
    )

    task(
        "updater-run",
        desc = "Launches the Knossos updater",
        deps = ["updater-build"],
        cmds = ["build/updater/updater"],
    )

    task(
        "uploader-build",
        desc = "Builds the Knossos uploader",
        deps = [],
        env = {
            "CC": "gcc",
            "CXX": "g++",
            "CGO_LDFLAGS": get_libarchive_flags(),
        },
        cmds = [
            "mkdir -p build/updater",
            "cd packages/updater",
            "go build -o ../../build/updater/uploader%s ./cmd/uploader" % binext,
        ],
    )

    task(
        "clean",
        desc = "Delete all generated files",
        deps = ["build-tool", "database-clean"],
        ignore_exit = True,
        cmds = [
            "rm -rf build/*",
            "rm -f packages/api/api/**/*.{ts,go}",
            "rm -f packages/api/client/**/*.go",
            "rm -rf packages/{client-ui,front}/dist",
            "rm -f packages/{client-ui,front}/gen/*",
        ],
    )

    for name in ("libknossos", "client"):
        task(
            "%s-clean" % name,
            desc = "Delete all generated files from the %s package" % name,
            deps = ["build-tool"],
            ignore_exit = True,
            cmds = [
                "rm -rf build/%s" % name,
            ],
        )
