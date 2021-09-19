#!/bin/sh

set -e

cd "$(dirname "$0")"/../..
mkdir -p build/libinnoextract
cd build/libinnoextract

export PATH="/mingw64/bin:$PATH"

if [ ! -f build.ninja ]; then
    args=(
        -DCMAKE_BUILD_TYPE=Release
        -DCMAKE_EXPORT_COMPILE_COMMANDS=ON
        -Wno-dev
        -DBoost_NO_WARN_NEW_VERSIONS=ON
        -DUSE_LTO=OFF # LTO is buggy with GNU's ld

        ../../packages/libinnoextract
    )

    cmake -GNinja "${args[@]}"
fi

ninja
