load("options.star", "msys2_path")

static_deps = option("static_deps", "true", "whether to link against static dependency libs (only on Linux and macOS)")

yarn_path = resolve_path("//", read_yaml("//.yarnrc.yml", "yarnPath"))

def yarn(*args):
    if len(args) == 1 and type(args[0]) == "string":
        args = tuple(args[0].split(" "))

    return ("node", yarn_path) + args

def protoc(args, go=None, twirp=None, ts=None):
    """A helper to construct protoc commands.

    Args:
      args: parameters to pass to protoc
      go: the path to be used for --go_out
      twirp: the path to be used for --twirp_out
      ts: the path to be used for --ts_out
    Returns:
      the complete command
    """
    cmd = "protoc -Idefinitions %s" % args
    if go:
        cmd += " --go_opt=paths=source_relative --go_out=%s" % go

    if twirp:
        cmd += " --twirp_out=%s" % twirp

    if ts:
        cmd += " --ts_opt long_type_number --ts_out=%s" % ts

    return cmd

def cmake_task(name, desc = "", inputs = [], outputs = [], script = None, windows_script = None, unix_script = None):
    """A wrapper around task() that sets common options for CMake projects

    Args:
      name: task name
      desc: a description for the task
      inputs: a list of files that will be processed by this task
      outputs: a list of files that will be created by this task
      script: the script that will call CMake
      windows_script: If script is None, this script will be used instead on Windows.
      unix_script: If script is None, this script will be used instead on Unix (Linux / macOS).
    """
    if OS == "windows":
        if not script:
            script = windows_script

        # Only declare this task if necessary
        if not hastask("bootstrap-mingw64"):
            task(
                "bootstrap-mingw64",
                desc = "Runs first-time setup for MSYS2",
                deps = ["fetch-deps"],
                base = msys2_path,
                skip_if_exists = [
                    "mingw64/bin/gcc.exe",
                    "mingw64/bin/cmake.exe",
                    "mingw64/bin/SDL2.dll",
                ],
                cmds = [
                    'usr/bin/bash -lc true',
                    'usr/bin/bash -lc "pacman --noconfirm -Syu"',
                    'usr/bin/bash -lc \'pacman --noconfirm -Syu --needed $(cat "$(cygpath -w "%s")")\'' % resolve_path('//msys2-packages.txt'),
                ],
            )

        task(
            name,
            desc = desc + " (uses MSYS2)",
            deps = ["fetch-deps", "bootstrap-mingw64"],
            inputs = inputs + [script],
            outputs = outputs,
            env = {
                # make sure CMake uses MSYS2's GCC
                "CC": "gcc",
                "CXX": "g++",
            },
            cmds = [
                ("cd", resolve_path(msys2_path)),
                ("usr/bin/bash", "-lc", '"$(cygpath "%s")"' % resolve_path(script)),
            ],
        )
    else:
        if not script:
            script = unix_script

        task(
            name,
            desc = desc,
            deps = ["fetch-deps"],
            inputs = inputs + [script],
            outputs = outputs,
            cmds = [
                ("sh", resolve_path(script)),
            ],
        )

def find_static_lib(names, display_name = None):
    """A helper to find libxyz.a files on most distros.

    Args:
      names: a list of possible library names (i.e. ["libz", "zlib"])
      display_name (optional): the name to use in log messages, defaults to the first item in names
    Returns:
      absolute path to the .a file
    """
    if OS not in ("linux", "darwin"):
        error("find_static_lib() is only supported on Linux and macOS.")

    if not display_name:
        display_name = names[0]

    for name in names:
        so_path = lookup_lib(name + ".so")
        if so_path:
            a_path = so_path.replace(".so", ".a")
            if isfile(a_path):
                return a_path

    error("Could not find static library for %s! Please make sure it's installed." % display_name)
    return None

def find_library(names, display_name = None):
    """A helper which either calls find_static_lib() or returns -l<libname> depending on static_deps.

    Args:
      names: a list of possible library names (i.e. ["libz", "zlib"])
      display_name (optional): the name to use in log messages, defaults to the first item in names
    Returns:
      absolute path to the .a file if static_deps else "-l<libname>"
    """

    if static_deps == "true":
        return find_static_lib(names, display_name)
    else:
        name = names[0]
        if name.startswith("lib"):
            name = name[3:]

        return "-l" + name

def get_golangci_flags():
    if getenv("CI") != "":
        return " --out-format=github-actions"
    else:
        return ""

# This is necessary because VSCode's clangd extension only supports a single compile_commands.json at the root
# of the project.
merge_compile_commands = task(
    "merge-compile-commands",
    desc = "Merges all compile_commands.json files into one",
    deps = ["build-tool"],
    base = "//",
    cmds = ["tool merge-compile-commands compile_commands.json build/*/compile_commands.json"],
)
