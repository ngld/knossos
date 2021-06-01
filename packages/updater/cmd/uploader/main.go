package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Masterminds/semver/v3"
)

func main() {
	args := flag.Args()
	if len(args) != 3 {
		fmt.Println("Usage: uploader <version> <platform> <release directory>")
		os.Exit(1)
	}

	version, err := semver.StrictNewVersion(args[0])
	if err != nil {
		fmt.Printf("Failed to parse version: %+v\n", err)
		os.Exit(1)
	}

	platform := args[1]
	if platform != "darwin" && platform != "windows" && platform != "linux" {
		fmt.Printf("Platform %s is not one of the accepted values (darwin, windows, linux).\n", platform)
		os.Exit(1)
	}

	reldir, err := filepath.Abs(args[2])
	if err != nil {
		fmt.Printf("Failed to proces release path: %+v\n", err)
		os.Exit(1)
	}

	info, err := os.Stat(reldir)
	if err != nil {
		fmt.Printf("Failed to access release directory: %+v\n", err)
		os.Exit(1)
	}

	if !info.IsDir() {
		fmt.Println("The release path does not point to a directory!")
		os.Exit(1)
	}
}
