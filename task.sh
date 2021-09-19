#!/bin/sh

set -e

cd "$(dirname "$0")"

needed_version="$(< .go-version)"
current_version="$(go version | cut -d " " -f 2 | cut -b 3-)"

if [ -d third_party/go/bin ]; then
    export PATH="$PWD/third_party/go/bin:$PATH"
fi

if ! command -v go > /dev/null 2>&1; then
    echo "Please install the Golang toolchain and run this script again. Make sure you install version $current_version."
    echo "Most likely, your distribution packages will outdated. If you're on Linux, go to https://golang.org/dl/ and grab"
    echo "the go${needed_version}.linux-amd64.tar.gz archive, extract it in the third_party directory so that the go binary"
    echo "ends up in third_party/go/bin/go."
    echo "If you're on macOS, you can either install go through Homebrew (if they aren't lagging behind) or click the above"
    echo "link and run the go${needed_version}.darwin-<amd64 or arm64>.pkg installer."
    echo ""
    echo "Finally, if you'd rather use a version manager (similar to nodenv, nvm, pyenv, ...), I'd recommend goenv:"
    echo " https://github.com/syndbg/goenv"
    echo
    echo "It should automatically pick up the .go-version file in this directory and select the correct version."
    exit 1
fi

if [ ! "$current_version" = "$needed_version" ] && [ ! -f .skip-go-version-check ]; then
    echo "Detected Go version $current_version but expected $needed_version."
    echo ""
    echo "Most likely, you installed Go through an outdated distro package. If you're on Linux, you can go"
    echo "to https://golang.org/dl/, grab the go${needed_version}.linux-amd64.tar.gz archive and extract it in"
    echo "the third_party directory so that the go binary ends up in third_party/go/bin/go. This script will then use"
    echo "that version instead."
    echo "If you're on macOS, you can do follow the same steps but use the go${needed_version}.darwin-<amd64 or arm64>.tar.gz"
    echo "instead."
    echo ""
    echo "If you'd rather use a version manager (similar to nodenv, nvm, pyenv, ...) to automate this, I'd recommend goenv:"
    echo " https://github.com/syndbg/goenv"
    echo
    echo "It should automatically pick up the .go-version file in this directory and select the correct version."
    echo
    echo "Finally, if you'd rather skip this check and use your already installed version, run \"touch .skip-go-version-check\"."
    exit 1
fi

if [ ! -f .tools/tool ]; then
    (
        cd packages/build-tools
        echo "Building build-tools..."
        go build -o ../../.tools/tool
    )
fi

exec ./.tools/tool task "$@"
