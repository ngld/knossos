load("helpers.star", "cmake_task", "find_library", "get_golangci_flags", "merge_compile_commands", "yarn")
load("options.star", "build")

kn_args = option("client_args", "", help = "The parameters to pass to Knossos in the client-run target")
local_nebula = option("use_local_nebula", "true" if build == "Debug" else "false", help = "Use localhost:8200 instead of nu.fsnebula.org (enabled by default for Debug builds)") == "true"

def knossos_configure(binext, libext, generator):
    res_dir = ""
    if OS == "darwin":
        res_dir = "Knossos.app/Contents/Frameworks/Chromium Embedded Framework.framework/Resources/"

    # Always build as release because the perfomance hit for debug CEF builds is pretty noticable
    # and often pointless since we usually don't touch the C++ code.
    cef_build = "Release"

    task(
        "client-ui-build",
        desc = "Builds the assets for Nebula's client UI",
        deps = ["build-tool", "fetch-deps"],
        base = "packages/client-ui",
        inputs = ["src/**/*.{ts,tsx,js,css}"],
        outputs = ["../../build/client/launcher/%s/%sui.kar" % (cef_build, res_dir)],
        cmds = [
            yarn("webpack --env production --color --progress"),
            "cp src/splash.html src/resources/logo.webm dist/prod",
            'tool pack-kar "../../build/client/launcher/%s/%sui.kar" dist/prod' % (cef_build, res_dir),
        ],
    )

    task(
        "client-ui-watch",
        hidden = True,
        deps = ["fetch-deps"],
        base = "packages/client-ui",
        cmds = [yarn("client:watch")],
    )

    cmake_task(
        "libarchive-build",
        desc = "Builds libarchive with CMake",
        inputs = ["third_party/libarchive/libarchive/**/*.{c,h}"],
        outputs = ["build/libarchive/libarchive/*.a"],
        windows_script = "packages/libarchive/msys2-build.sh",
        unix_script = "packages/libarchive/unix-build.sh",
    )

    libkn_ldflags = ""

    # platform specific filename for libarchive
    if OS == "windows":
        libkn_ldflags += str(resolve_path("build/libarchive/libarchive/libarchive_static.a"))
    else:
        libkn_ldflags += str(resolve_path("build/libarchive/libarchive/libarchive.a"))

    if OS == "darwin":
        # look for liblzma in the lib directory from homebrew's xz package
        # darwin's ld doesn't understand --no-undefined so skip it there
        libkn_ldflags += " -L/usr/local/opt/xz/lib"

    if OS != "darwin":
        libkn_ldflags += " -Wl,--no-undefined"

    if OS != "linux":
        libkn_ldflags += " -liconv -llzma -lzstd -lz"
    else:
        libkn_ldflags += " " + " ".join([
            find_library(["liblzma"]),
            find_library(["libzstd"]),
            find_library(["libz", "zlib"], "zlib"),
        ])

    task(
        "libknossos-lint",
        desc = "Lints libknossos with golangci-lint",
        deps = ["fetch-deps", "proto-build", "libarchive-build"],
        base = "packages/libknossos",
        env = {
            # cgo only supports gcc, make sure it doesn't try to use a compiler meant for our other packages
            "CC": "gcc",
            "CGO_LDFLAGS": libkn_ldflags,
        },
        cmds = ["golangci-lint run" + get_golangci_flags()],
    )

    libkn_flags = ""
    libkn_goldflags = ""
    libkn_defines = {}
    if build == "Release":
        libkn_flags += " -tags release -trimpath"
        libkn_goldflags += " -s -w"
        libkn_defines["releaseBuild"] = "true"

    if local_nebula:
        libkn_defines["TwirpEndpoint"] = "http://localhost:8200/twirp"
        libkn_defines["SyncEndpoint"] = "http://localhost:8200/sync"

    for k, v in libkn_defines.items():
        libkn_goldflags += " -X github.com/ngld/knossos/packages/libknossos/pkg/api.%s=%s" % (k, v)

    libkn_flags += " -ldflags '%s'" % libkn_goldflags

    task(
        "libknossos-build",
        desc = "Builds libknossos (client-side, non-UI logic)",
        deps = ["build-tool", "proto-build", "libarchive-build"],
        base = "packages/libknossos",
        inputs = [
            "../../.tools/tool%s" % binext,
            "**/*.go",
            "../libarchive/**/*.go",
        ],
        outputs = [
            "../../build/libknossos/libknossos%s" % libext,
            "../../build/libknossos/dynknossos.{h,cc}",
        ],
        env = {
            # cgo only supports gcc, make sure it doesn't try to use a compiler meant for our other packages
            "CC": "gcc",
            "CGO_LDFLAGS": libkn_ldflags,
        },
        cmds = [
            "go build %s -o ../../build/libknossos/libknossos%s -buildmode c-shared ./api" % (libkn_flags, libext),
            "tool gen-dyn-loader ../../build/libknossos/libknossos.h ../../build/libknossos/dynknossos.h",
        ],
    )

    if generator == "Ninja":
        build_cmd = "ninja knossos"
    elif generator == "Unix Makefiles":
        build_cmd = "make -j4 knossos"
    else:
        build_cmd = "cmake --build ."

    task(
        "client-build",
        desc = "Builds the Knossos client",
        deps = ["libarchive-build", "libknossos-build"],
        cmds = [
            "mkdir -p build/client",
            "cd build/client",
            """
    if [ ! -f CMakeCache.txt ] || [ ! -f compile_commands.json ]; then
        cmake -G"{generator}" -DCMAKE_BUILD_TYPE={cef_build} -DCMAKE_EXPORT_COMPILE_COMMANDS=1 ../../packages/client
    fi
    """.format(generator = generator, cef_build = cef_build),
            merge_compile_commands,
            build_cmd,
        ],
    )

    if OS == "darwin":
        kn_bin = "./launcher/%s/Knossos.app/Contents/MacOS/knossos" % cef_build
    else:
        kn_bin = "./launcher/%s/knossos" % cef_build

    task(
        "client-run",
        desc = "Launches Knossos",
        deps = ["client-build", "client-ui-build"],
        base = "build/client",
        cmds = ["%s %s" % (kn_bin, kn_args)],
    )

    libkn_path = resolve_path("build/libknossos/libknossos%s" % libext)
    task(
        "client-run-dev",
        hidden = True,
        base = "build/client",
        deps = ["client-build"],
        cmds = ['%s --url="http://localhost:8080/" --libkn="%s"' % (kn_bin, libkn_path)],
    )

    task(
        "client-ws-build",
        hidden = True,
        deps = ["proto-build", "libarchive-build"],
        base = "packages/libknossos",
        inputs = [
            "../../.tools/tool%s" % binext,
            "**/*.go",
            "../libarchive/**/*.go",
        ],
        outputs = [
            "../../build/libknossos/dev-server%s" % binext,
            "../../build/libknossos/dynknossos.{h,cc}",
        ],
        env = {
            # cgo only supports gcc, make sure it doesn't try to use a compiler meant for our other packages
            "CC": "gcc",
            "CGO_LDFLAGS": libkn_ldflags,
        },
        cmds = [("go", "build", "-o", "../../build/libknossos/dev-server%s" % binext, "./dev-server")],
    )

    task(
        "client-ws-run",
        desc = "Launch Knossos WS server",
        deps = ["client-ws-build"],
        base = "packages/libknossos",
        cmds = ["../../build/libknossos/dev-server"],
    )

    task(
        "client-watch",
        desc = "Launch Knossos, recompile and restart after source changes",
        # run fetch-deps before we launch modd to make sure that it doesn't trigger
        # two parallel fetch-deps tasks
        deps = ["install-tools", "fetch-deps"],
        cmds = ["modd -f modd_client.conf"],
    )
