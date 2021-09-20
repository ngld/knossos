#!/bin/sh

set -e

cd "$(dirname "$0")"/../..
mkdir -p build/libarchive
cd build/libarchive

if [ ! -f Makefile ]; then
    link_opts=""
    if [ -f /usr/local/opt/xz/lib/liblzma.a ]; then
        link_opts="$link_opts -DLIBLZMA_LIBRARY=/usr/local/opt/xz/lib/liblzma.a"
    fi

    if [ -f /usr/local/opt/zstd/lib/libzstd.a ]; then
        link_opts="$link_opts -DZSTD_LIBRARY=/usr/local/opt/zstd/lib/libzstd.a"
    fi

    cmake -G"Unix Makefiles" \
        -DCMAKE_BUILD_TYPE=Release \
        -DCMAKE_EXPORT_COMPILE_COMMANDS=ON \
        -Wno-dev \
        -DENABLE_ACL=OFF \
        -DENABLE_BZip2=OFF \
        -DENABLE_CNG=OFF \
        -DENABLE_CPIO=OFF \
        -DENABLE_EXPAT=OFF \
        -DENABLE_LIBXML2=OFF \
        -DENABLE_LZ4=OFF \
        -DENABLE_OPENSSL=OFF \
        -DENABLE_PCREPOSIX=OFF \
        -DENABLE_TAR=OFF \
        -DENABLE_TEST=OFF \
        -DENABLE_CAT=OFF \
        $link_opts \
        ../../third_party/libarchive
fi

make -j$(nproc) archive_static
