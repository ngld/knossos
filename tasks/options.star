build = option("build", "Release", help = "Whether to build a Debug or Release build")
msys2_path = option("msys2_path", "//third_party/msys64", help = "The path to your MSYS2 installation. Only used on Windows. " +
                                                                 "Defaults to the bundled MSYS2 directory")
generator_opt = option("generator", "", help = "The CMake generator to use. Defaults to ninja if available. " +
                                               "Please note that on Windows you'll have to run the vcvarsall.bat if you don't choose a Visual Studio generator")

