load("helpers.star", "yarn", "find_library", "cmake_task", "merge_compile_commands", "get_golangci_flags")
load("options.star", "build")

kn_args = option("client_args", "", help = "The parameters to pass to Knossos in the client-run target")

def knossos_configure(binext, libext, generator):
    res_dir = ""
    if OS == "darwin":
        res_dir = "Knossos.app/Contents/Frameworks/Chromium Embedded Framework.framework/Resources/"

    task(
        "client-ui-build",
        desc = "Builds the assets for Nebula's client UI",
        deps = ["build-tool", "fetch-deps"],
        base = "packages/client-ui",
        inputs = ["src/**/*.{ts,tsx,js,css}"],
        outputs = ["../../build/client/launcher/%s/%sui.kar" % (build, res_dir)],
        cmds = [
            yarn("webpack --env production --color --progress"),
            'tool pack-kar "../../build/client/launcher/%s/%sui.kar" dist/prod' % (build, res_dir),
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
            "go build -o ../../build/libknossos/libknossos%s -buildmode c-shared ./api" % libext,
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
        cmake -G"{generator}" -DCMAKE_BUILD_TYPE={build} -DCMAKE_EXPORT_COMPILE_COMMANDS=1 ../../packages/client
    fi
    """.format(generator = generator, build = build),
            merge_compile_commands,
            build_cmd,
        ],
    )

    if OS == "darwin":
        kn_bin = "./launcher/%s/Knossos.app/Contents/MacOS/knossos" % build
    else:
        kn_bin = "./launcher/%s/knossos" % build

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
        "client-watch",
        desc = "Launch Knossos, recompile and restart after source changes",
        # run fetch-deps before we launch modd to make sure that it doesn't trigger
        # two parallel fetch-deps tasks
        deps = ["install-tools", "fetch-deps"],
        cmds = ["modd -f modd_client.conf"],
    )
