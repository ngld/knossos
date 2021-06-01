package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rotisserie/eris"
)

type manifest struct {
	Added    map[string]string
	Modified map[string]string
	Removed  []string
	Version  int
}

func listDirContents(dir, prefix string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			children, err := listDirContents(filepath.Join(dir, entry.Name()), prefix)
			if err != nil {
				return nil, err
			}

			files = append(files, children...)
		} else if entry.Type().IsRegular() {
			itemPath := filepath.Join(dir, entry.Name())
			itemPath = strings.TrimPrefix(itemPath, prefix)
			files = append(files, filepath.ToSlash(itemPath))
		}
	}

	return files, nil
}

func main() {
	args := os.Args[1:]
	if len(args) != 3 {
		fmt.Println("Usage: differ <old path> <new path> <output>")
		os.Exit(1)
	}

	fmt.Println("Building old file list")
	oldFiles, err := listDirContents(args[0], strings.TrimRight(args[0], "/")+"/")
	if err != nil {
		fmt.Printf("Failed to retrieve old file list: %+v\n", err)
		os.Exit(1)
	}

	fmt.Println("Building new file list")
	newFiles, err := listDirContents(args[1], strings.TrimRight(args[1], "/")+"/")
	if err != nil {
		fmt.Printf("Failed to retrieve new file list: %+v\n", err)
		os.Exit(1)
	}

	fmt.Println("Creating output directory")
	outDir := args[2]
	err = os.MkdirAll(outDir, 0770)
	if err != nil {
		fmt.Printf("Failed to create %s: %+v\n", outDir, err)
	}

	fmt.Println("Comparing")
	added := make([]string, 0)
	removed := make([]string, 0)
	modified := make([]string, 0)
	hashCache := make(map[string]string)

	sort.Strings(oldFiles)
	sort.Strings(newFiles)
	newIdx := 0
	for oldIdx := 0; oldIdx < len(oldFiles); oldIdx++ {
		if oldIdx >= len(newFiles) {
			removed = append(removed, oldFiles[oldIdx])
			continue
		}

		if oldFiles[oldIdx] < newFiles[newIdx] {
			removed = append(removed, oldFiles[oldIdx])
		}

		for oldFiles[oldIdx] > newFiles[newIdx] {
			added = append(added, newFiles[newIdx])
			newIdx++
		}

		if oldFiles[oldIdx] == newFiles[newIdx] {
			oldPath := filepath.Join(args[0], oldFiles[oldIdx])
			newPath := filepath.Join(args[1], oldFiles[newIdx])

			fmt.Printf("Checking %s\n", oldFiles[oldIdx])

			// TODO Zucchini
			hash := sha256.New()
			f, err := os.Open(oldPath)
			if err != nil {
				fmt.Printf("Failed to open %s: %+v\n", oldPath, err)
				os.Exit(1)
			}

			_, err = io.Copy(hash, f)
			if err != nil {
				f.Close()
				fmt.Printf("Failed to read %s: %+v\n", oldPath, err)
				os.Exit(1)
			}
			f.Close()

			oldHash := hash.Sum(nil)
			hash.Reset()
			f, err = os.Open(newPath)
			if err != nil {
				fmt.Printf("Failed to open %s: %+v\n", newPath, err)
				os.Exit(1)
			}

			_, err = io.Copy(hash, f)
			if err != nil {
				f.Close()
				fmt.Printf("Failed to read %s: %+v\n", newPath, err)
				os.Exit(1)
			}
			f.Close()

			newHash := hash.Sum(nil)
			if !bytes.Equal(oldHash, newHash) {
				modified = append(modified, oldFiles[oldIdx])
				hashCache[oldFiles[oldIdx]] = hex.EncodeToString(newHash)
			}

			newIdx++
		}

		if oldIdx == len(oldFiles)-1 {
			for newIdx++; newIdx < len(newFiles); newIdx++ {
				added = append(added, newFiles[newIdx])
			}
		}
	}

	manifest := manifest{
		Version:  1,
		Added:    make(map[string]string),
		Modified: make(map[string]string),
		Removed:  removed,
	}

	hash := sha256.New()
	for _, path := range added {
		hash.Reset()
		fmt.Println("Hashing", path)
		f, err := os.Open(filepath.Join(args[1], path))
		if err != nil {
			fmt.Printf("Failed to open %s: %+v\n", path, err)
			os.Exit(1)
		}

		_, err = io.Copy(hash, f)
		if err != nil {
			f.Close()
			fmt.Printf("Failed to read %s: %+v\n", path, err)
			os.Exit(1)
		}

		manifest.Added[path] = hex.EncodeToString(hash.Sum(nil))

		destPath := filepath.Join(outDir, manifest.Added[path])
		_, err = os.Stat(destPath)
		if err != nil && !eris.Is(err, os.ErrNotExist) {
			f.Close()
			fmt.Printf("Failed to check %s: %+v\n", destPath, err)
			os.Exit(1)
		}

		if eris.Is(err, os.ErrNotExist) {
			dest, err := os.Create(destPath)
			if err != nil {
				f.Close()
				fmt.Printf("Failed to create %s: %+v\n", destPath, err)
				os.Exit(1)
			}

			_, err = f.Seek(0, io.SeekStart)
			if err != nil {
				f.Close()
				dest.Close()
				fmt.Printf("Failed to seek in %s: %+v\n", path, err)
				os.Exit(1)
			}

			_, err = io.Copy(dest, f)
			if err != nil {
				f.Close()
				dest.Close()
				fmt.Printf("Failed to copy %s to %s: %+v\n", path, destPath, err)
				os.Exit(1)
			}

			dest.Close()
		}

		f.Close()
	}

	for _, path := range modified {
		manifest.Modified[path] = hashCache[path]

		srcPath := filepath.Join(args[1], path)
		destPath := filepath.Join(outDir, hashCache[path])

		_, err = os.Stat(destPath)
		if !eris.Is(err, os.ErrNotExist) {
			if err != nil {
				fmt.Printf("Failed to check %s: %+v\n", destPath, err)
				os.Exit(1)
			}

			continue
		}

		f, err := os.Open(srcPath)
		if err != nil {
			fmt.Printf("Failed to open %s: %+v\n", srcPath, err)
			os.Exit(1)
		}

		dest, err := os.Create(destPath)
		if err != nil {
			f.Close()
			fmt.Printf("Failed to create %s: %+v\n", destPath, err)
			os.Exit(1)
		}

		_, err = io.Copy(dest, f)
		if err != nil {
			f.Close()
			dest.Close()
			fmt.Printf("Failed to copy %s to %s: %+v", srcPath, destPath, err)
			os.Exit(1)
		}

		f.Close()
		dest.Close()
	}

	f, err := os.Create(filepath.Join(outDir, "manifest.json"))
	if err != nil {
		fmt.Printf("Failed to create manifest: %+v\n", err)
		os.Exit(1)
	}

	encoded, err := json.Marshal(&manifest)
	if err != nil {
		f.Close()
		fmt.Printf("Failed to encode manifest: %+v\n", err)
		os.Exit(1)
	}

	_, err = f.Write(encoded)
	if err != nil {
		f.Close()
		fmt.Printf("Failed to write manifest: %+v\n", err)
		os.Exit(1)
	}

	f.Close()
}
