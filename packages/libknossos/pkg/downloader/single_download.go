package downloader

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"hash"
	"io"
	"math/rand"
	"net/http"
	"os"
	"sort"
	"time"

	"github.com/ngld/knossos/packages/libknossos/pkg/api"
	"github.com/rotisserie/eris"
)

func DownloadSingle(ctx context.Context, filepath string, mirrors []string, checksum []byte, retries int, progressCb ProgressCallback) error {
	// TODO: Implement a better algorithm for URL ordering, preferably by
	// ranking based on mirror preference but with some randomness so we don't
	// send all requests to the same mirror...
	// For now we just shuffle the URLs randomly to evenly distribute the load
	// across mirrors.
	urls := make([]string, len(mirrors))
	copy(urls, mirrors)
	rand.Shuffle(len(urls), sort.StringSlice(urls).Swap)

	f, err := os.Create(filepath)
	if err != nil {
		return eris.Wrapf(err, "failed to create %s", filepath)
	}
	defer f.Close()

	var hasher hash.Hash
	if checksum != nil {
		hasher = sha256.New()
	}

	progress := int64(0)
	filesize := int64(0)
	var speedTracker *api.SpeedTracker

	if progressCb != nil {
		speedTracker = api.NewSpeedTracker()
		done := false
		defer func() {
			done = true
		}()

		go func() {
			lastPos := progress
			for !done {
				speedTracker.Track(int(progress - lastPos))

				if filesize > 0 {
					progressCb(float32(progress)/float32(filesize), speedTracker.GetSpeed())
				}

				time.Sleep(300 * time.Millisecond)
			}
		}()
	}

	var lastError error
	midx := -1
	success := false
	buffer := make([]byte, 4*1024)
	for try := 0; try < retries; try++ {
		midx++
		if midx >= len(urls) {
			midx = 0
		}

		req, err := http.NewRequest("GET", urls[midx], nil)
		if err != nil {
			return eris.Wrapf(err, "failed to build request for %s", urls[midx])
		}

		req.Header.Set("User-Agent", userAgent)
		if progress > 0 {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-", progress))
		}

		if lastError == nil {
			api.Log(ctx, api.LogInfo, "Downloading %s", urls[midx])
		} else {
			api.Log(ctx, api.LogWarn, "Failed (%v), trying again with %s", lastError, urls[midx])
		}

		resp, err := dlClient.Do(req)
		if err != nil {
			lastError = eris.Wrapf(err, "failed to fetch %s", urls[midx])
			continue
		}

		if resp.StatusCode != 200 && resp.StatusCode != 206 {
			api.Log(ctx, api.LogError, "%s failed with status %d", urls[midx], resp.StatusCode)
			lastError = eris.Errorf("%s failed with status %d", urls[midx], resp.StatusCode)
			resp.Body.Close()
			continue
		}

		if resp.StatusCode == 200 {
			if filesize <= 0 {
				filesize = resp.ContentLength
			}

			_, err = f.Seek(0, io.SeekStart)
			if err != nil {
				resp.Body.Close()
				return eris.Wrapf(err, "failed to seek in %s", filepath)
			}

			progress = 0
			if hasher != nil {
				hasher.Reset()
			}
		}

		if filesize == 0 {
			filesize = resp.ContentLength
		}

		if filesize > 0 {
			if resp.StatusCode == 200 || (resp.ContentLength+progress) != filesize {
				api.Log(ctx, api.LogWarn, "%s has unexpected size %d != %d", urls[midx], resp.ContentLength, filesize)
			}
		}

		for {
			read, err := resp.Body.Read(buffer)
			// Read() can return io.EOF for the last available block.
			// This means that we have to process the received data before we can take a look at the error.
			if read > 0 {
				_, err := f.Write(buffer[:read])
				if err != nil {
					resp.Body.Close()
					return eris.Wrap(err, "failed to write")
				}

				progress += int64(read)
				if hasher != nil {
					// hash.Hash's Write() never fails which means we don't have to check it's return values
					hasher.Write(buffer[:read])
				}
			}

			if err != nil {
				if eris.Is(err, io.EOF) {
					success = true
					break
				}
				lastError = eris.Wrap(err, "failed to read")
				break
			}

			if ctx.Err() != nil {
				resp.Body.Close()
				return ctx.Err()
			}
		}

		resp.Body.Close()

		if success {
			break
		}
	}

	if !success {
		return lastError
	}

	pos, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		return eris.Wrap(err, "failed to check file position")
	}

	if pos != progress {
		return eris.Errorf("internal consistency error for %s: file position is %d but received %d bytes", urls[midx], pos, progress)
	}

	if filesize > 0 && pos != filesize {
		return eris.Errorf("%s was %d bytes but expected %d", urls[midx], pos, filesize)
	}

	if hasher != nil {
		fileSum := hasher.Sum(nil)
		if !bytes.Equal(fileSum, checksum) {
			return eris.Errorf("%s failed due to a checksum mismatch (%s != %s)", urls[midx], fileSum, checksum)
		}
	}

	return nil
}
