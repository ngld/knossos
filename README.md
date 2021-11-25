# Monorepo for Knossos and Nebula development

## DISCLAIMER: This code is in pre-alpha stage

I'm currently rewriting Knossos from scratch and as a result, this repository
currently doesn't contain a functional mod manager. If you're just trying to
run Knossos, please take a look at the [old repository](https://github.com/ngld/old-knossos).

This repository contains the unfinished code for the next version and doesn't
contain essential features like mod installation. You're welcome to test the
software in its current state. Just please be aware of the limitations.

I'll remove this notice once the basic mod manager functionality (mod install,
update, remove, ...) has been implemented.

## Overview

Most of the source code is in the `packages` subdirectory. It also contains
further documentation for the various packages.

## Build preparation

### Windows

Before you compile this project, make sure you have [Visual Studio 2019][vs] or
the [Build Tools for Visual Studio 2019][build-tools] installed. The latter only
provide CLI tools which should be enough to build this project.

If you're still on Windows 7 or haven't updated Windows 10 in over a year, you'll
also have to install the [Golang toolchain][go].

Once you're set up, double-click the `open_shell.bat` file to open a terminal
in this folder or open your favorite terminal and navigate here manually.  
NOTE: I highly recommend installing [Windows Terminal][wt] for a better
terminal experience on Windows. If you don't like the Windows Store, you can
also [manually download and install the tool][wt-releases].

Continue with the general instructions below.

### Linux

You'll need the following:

* go (the package is sometimes called golang)
* CMake
* Docker (only for Nebula)
* GCC
* GTK 3
* NSS
* Boost
* OpenAL
* liblzma
* libzstd
* SDL2
* ninja (optional)
* zlib

Once you've installed these dependencies, follow the general instructions and
replace the `task.ps1` command with `./task.sh`.

### macOS

You'll need the following:

* liblzma installed through Homebrew (`/usr/local/opt/xz/lib/liblzma.a`)

Once you've installed these dependencies, follow the general instructions and
replace the `task.ps1` command with `./task.sh`.

## Build instructions

Enter `task.ps1 configure -o` to get a list of available options and their default
values. The first time you run this command, the `task.ps1` script will compile the
build system before launching it.
To modify the listed options, run `task.ps1 configure option1=value option2=...`.
If you don't want to modify the options, just run `task.ps1 configure` without any
further parameters. This will run a few platform checks necessary before we can
run any build targets.

To get a list of available build targets, run `task.ps1 -l`. `client-build` and
`client-run` are probably the most interesting. To build a target, run
`task.ps1 <target>`. It's pretty similar to make: You can pass multiple targets,
dependencies are automatically built whenever necessary. If a target's source
files haven't changed since the last build, it will be skipped.

[vs]: https://visualstudio.microsoft.com/thank-you-downloading-visual-studio/?sku=Community&rel=16
[build-tools]: https://visualstudio.microsoft.com/thank-you-downloading-visual-studio/?sku=BuildTools&rel=16
[go]: https://golang.org/dl/
[wt]: https://aka.ms/terminal
[wt-releases]: https://github.com/microsoft/terminal/releases
