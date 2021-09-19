package main

import (
	"fmt"
	"os"

	"github.com/ngld/knossos/packages/libinnoextract"
)

func progressCb(progress float32, message string) {
	fmt.Printf("[%3f]: %s\n", progress, message)
}

func logCb(level libinnoextract.LogLevel, message string) {
	fmt.Printf("%10s: %s\n", level.String(), message)
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: innoextract-test <installer> <dest>")
		os.Exit(2)
	}

	err := libinnoextract.ExtractInstaller(os.Args[1], os.Args[2], progressCb, logCb)
	if err != nil {
		fmt.Println("Error:")
		panic(err)
	}

	fmt.Println("Done")
}
