package main

import (
	"context"
	"encoding/hex"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/minio/sha256-simd"

	"github.com/ngld/knossos/packages/libarchive"
	"github.com/ngld/knossos/packages/server/pkg/importer"
	"github.com/rotisserie/eris"
	"github.com/rs/zerolog/log"
)

func buildFilelist(ctx context.Context, archiveMeta importer.KnArchive, archiveID int32, renames map[string]string) ([]importer.KnFile, error) {
	result := make([]importer.KnFile, 0)

	url := ""
	for _, item := range archiveMeta.URLs {
		if strings.HasPrefix(item, "https://dl.fsnebula.org/") {
			url = item
			break
		}
	}

	if url == "" {
		url = archiveMeta.URLs[0]
	}

	log.Info().Msgf("Downloading %s", url)
	proc := exec.Command("curl", "-Lo", "archive", url)
	proc.Stdout = os.Stdout
	proc.Stderr = os.Stderr
	proc.Stdin = nil

	defer os.Remove("archive")
	err := proc.Run()
	if err != nil {
		return nil, eris.Wrap(err, "download failed")
	}

	log.Info().Msg("Hashing")
	archive, err := libarchive.OpenArchive("archive")
	if err != nil {
		return nil, eris.Wrap(err, "failed to open archive")
	}

	hasher := sha256.New()
	buffer := make([]byte, 128*1024)

	for archive.Next() == nil {
		if archive.Entry.Size < 1 {
			continue
		}

		hasher.Reset()

		for {
			n, err := archive.Read(buffer)
			if err != nil {
				if eris.Is(err, io.EOF) {
					break
				}

				return nil, eris.Wrapf(err, "failed to read %s from archive", archive.Entry.Pathname)
			}

			_, err = hasher.Write(buffer[n:])
			if err != nil {
				return nil, eris.Wrapf(err, "failed to calculate hash for %s", archive.Entry.Pathname)
			}
		}

		newName, present := renames[archive.Entry.Pathname]
		if !present {
			newName = archive.Entry.Pathname
		}

		result = append(result, importer.KnFile{
			OrigName:  archive.Entry.Pathname,
			Filename:  newName,
			Archive:   archiveMeta.Filename,
			ArchiveID: archiveID,
			Checksum:  importer.KnChecksum{"sha256", string(hex.EncodeToString(hasher.Sum(nil)))},
		})
	}

	err = archive.Error()
	if err != nil {
		return nil, eris.Wrap(err, "failed to read from archive")
	}

	return result, nil
}
