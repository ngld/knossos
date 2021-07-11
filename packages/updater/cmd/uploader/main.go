package main

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"

	"github.com/Masterminds/semver/v3"
	"github.com/ngld/knossos/packages/libarchive"
	"github.com/rotisserie/eris"
)

func addFolderToArchive(a *libarchive.ArchiveWriter, folder, prefix string) error {
	items, err := os.ReadDir(folder)
	if err != nil {
		return err
	}

	for _, item := range items {
		if item.IsDir() {
			err = addFolderToArchive(a, filepath.Join(folder, item.Name()), path.Join(prefix, item.Name()))
			if err != nil {
				return err
			}
		} else {
			fmt.Println(path.Join(prefix, item.Name()))
			f, err := os.Open(filepath.Join(folder, item.Name()))
			if err != nil {
				return eris.Wrapf(err, "failed to open %s", filepath.Join(folder, item.Name()))
			}

			size, err := f.Seek(0, io.SeekEnd)
			if err != nil {
				f.Close()
				return eris.Wrapf(err, "failed to determine size of %s", filepath.Join(folder, item.Name()))
			}

			_, err = f.Seek(0, io.SeekStart)
			if err != nil {
				f.Close()
				return eris.Wrapf(err, "failed to seek in %s", filepath.Join(folder, item.Name()))
			}

			mode := libarchive.ModeRegular | 0666
			stat, err := os.Stat(filepath.Join(folder, item.Name()))
			if err != nil {
				f.Close()
				return eris.Wrapf(err, "failed to stat %s", filepath.Join(folder, item.Name()))
			}

			if stat.Mode().Perm()&0100 != 0 {
				mode |= 0111
			}

			err = a.CreateFile(path.Join(prefix, item.Name()), mode, size)
			if err != nil {
				f.Close()
				return eris.Wrap(err, "failed to create archive entry")
			}

			_, err = io.Copy(a, f)
			if err != nil {
				f.Close()
				return eris.Wrapf(err, "failed to copy data from %s", filepath.Join(folder, item.Name()))
			}

			f.Close()
		}
	}

	return nil
}

func main() {
	args := os.Args[1:]
	if len(args) != 3 {
		fmt.Println("Usage: uploader <version> <platform> <release directory>")
		os.Exit(1)
	}

	_, err := semver.StrictNewVersion(args[0])
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

	a, err := libarchive.CreateArchive("knossos_layer_tmp.7z")
	if err != nil {
		fmt.Printf("Failed to create archive: %+v\n", err)
		os.Exit(1)
	}

	err = addFolderToArchive(a, reldir, "")
	if err != nil {
		fmt.Printf("Failed to compress files: %s\n", eris.ToString(err, true))
		os.Exit(1)
	}

	a.Close()

	fmt.Println("Uploading")
	err = uploadArchive("knossos_layer_tmp.7z", platform+"-"+args[0])
	if err != nil {
		fmt.Printf("Failed to upload archive: %s\n", eris.ToString(err, true))
		os.Exit(1)
	}

	fmt.Println("Done")
}
