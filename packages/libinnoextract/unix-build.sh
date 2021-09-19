#!/bin/sh

set -e

cd "$(dirname "$0")"/../..
mkdir -p build/libinnoextract
cd build/libinnoextract

if [ ! -f CMakeCache.txt ]; then
	link_opts=""
    if [ -f /usr/local/opt/xz/lib/liblzma.a ]; then
        link_opts="-DLIBLZMA_LIBRARY=/usr/local/opt/xz/lib/liblzma.a"
    fi

    args=(
        -DCMAKE_BUILD_TYPE=Release
        -DCMAKE_EXPORT_COMPILE_COMMANDS=ON
        -Wno-dev
        -DUSE_LTO=OFF # LTO is buggy with GNU's ld
		$link_opts
        ../../packages/libinnoextract
    )

    cmake -G"Unix Makefiles" "${args[@]}"
fi

make -j$(nproc)
