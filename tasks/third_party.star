load("helpers.star", "cmake_task", "find_library")
load("options.star", "msys2_path")

def third_party_configure(binext, libext, generator):
    libarchive_ldflags = ""

    # platform specific filename for libarchive
    if OS == "windows":
        libarchive_ldflags += str(resolve_path("build/libarchive/libarchive/libarchive_static.a"))
    else:
        libarchive_ldflags += str(resolve_path("build/libarchive/libarchive/libarchive.a"))

    if OS != "darwin":
        libarchive_ldflags += " -Wl,--no-undefined"

    if OS == "windows":
        # force static linking on windows
        libs = ["liblzma", "libzstd", "libz", "libiconv"]
        for lib in libs:
            libarchive_ldflags += " %s.a" % resolve_path(msys2_path, "mingw64/lib", lib)

        libarchive_ldflags = libarchive_ldflags.replace("\\", "/")
    elif OS == "darwin":
        if ARCH == "arm64":
            libarchive_ldflags += " -liconv -lz /opt/homebrew/opt/xz/lib/liblzma.a /opt/homebrew/opt/zstd/lib/libzstd.a"
        else:
            libarchive_ldflags += " -liconv -lz /usr/local/opt/xz/lib/liblzma.a /usr/local/opt/zstd/lib/libzstd.a"
    else:
        libarchive_ldflags += " " + " ".join([
            find_library(["liblzma"]),
            find_library(["libzstd"]),
            find_library(["libz", "zlib"], "zlib"),
        ])

    write_file("packages/libarchive/cgo_flags.go", """
package libarchive

// #cgo LDFLAGS: %s
import "C"
""" % libarchive_ldflags)

    cmake_task(
        "libarchive-build",
        desc = "Builds libarchive with CMake",
        inputs = ["third_party/libarchive/libarchive/**/*.{c,h}"],
        outputs = ["build/libarchive/libarchive/*.a"],
        windows_script = "packages/libarchive/msys2-build.sh",
        unix_script = "packages/libarchive/unix-build.sh",
    )

    libinno_task = cmake_task(
        "libinnoextract-cmake-build",
        hidden = True,
        desc = "Builds libinnoextract with CMake",
        inputs = ["third_party/innoextract/{cmake/*.cmake,src/**/*.{cpp,hpp}{,.in}}", "packages/libinnoextract/CMakeLists.txt"],
        outputs = ["build/libinnoextract/libinnoextract.*"],
        windows_script = "packages/libinnoextract/msys2-build.sh",
        unix_script = "packages/libinnoextract/unix-build.sh",
    )

    task(
        "libinnoextract-build",
        desc = "Builds libinnoextract",
        deps = ["install-tools", "libinnoextract-cmake-build"],
        base = "packages/libinnoextract",
        inputs = ["*.go", "**/*.go"],
        outputs = ["*_string.go"],
        cmds = ["go generate ."],
    )
