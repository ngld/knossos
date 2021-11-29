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
	"sync"
	"sync/atomic"
	"time"

	"github.com/ngld/knossos/packages/libknossos/pkg/api"
	"github.com/ngld/knossos/packages/libknossos/pkg/storage"
	"github.com/rotisserie/eris"
)

type QueueItem struct {
	Key      string
	Filepath string
	Mirrors  []string
	Checksum []byte
	Filesize int64
}

type ProgressCallback func(progress float32, speed float64)

type Queue struct {
	err                 error
	finishedLock        *sync.Cond
	activeLock          *sync.Cond
	result              *QueueItem
	ProgressCb          ProgressCallback
	queued              []*QueueItem
	finishedItems       []*QueueItem
	progress            []*uint32
	speedTracker        api.SpeedTracker
	MaxParallel         int
	active              int
	totalBytes          int
	Retries             int
	periodReceivedBytes uint32
	done                bool
}

var dlClient = http.Client{
	Timeout: 0,
}
var userAgent = fmt.Sprintf("Knossos %s (+https://fsnebula.org/knossos/)", api.Version)

func NewQueue(ctx context.Context, items []*QueueItem) (*Queue, error) {
	settings, err := storage.GetSettings(ctx)
	if err != nil {
		return nil, eris.Wrap(err, "failed to read settings")
	}

	q := &Queue{
		result:       nil,
		queued:       items,
		MaxParallel:  int(settings.MaxDownloads),
		Retries:      5,
		progress:     make([]*uint32, len(items)),
		activeLock:   sync.NewCond(&sync.Mutex{}),
		finishedLock: sync.NewCond(&sync.Mutex{}),
	}

	// Enforce sane default.
	if q.MaxParallel < 1 {
		q.MaxParallel = 3
	}

	for idx := range q.progress {
		q.progress[idx] = new(uint32)
	}

	for _, item := range items {
		if len(item.Mirrors) < 1 {
			panic(fmt.Sprintf("no URLs provided for item %s", item.Key))
		}
		q.totalBytes += int(item.Filesize)
	}

	return q, nil
}

func (q *Queue) Run(ctx context.Context) error {
	if q.done {
		panic("Called Run() on a finished queue")
	}

	q.activeLock.L.Lock()
	if q.active > 0 {
		q.activeLock.L.Unlock()
		panic("Called Run() on an already running queue")
	}
	q.activeLock.L.Unlock()

	if q.ProgressCb != nil {
		go q.updateProgressTicker()
	}

	for idx, item := range q.queued {
		if ctx.Err() != nil {
			q.done = true
			return eris.Wrap(ctx.Err(), "context error")
		}

		q.activeLock.L.Lock()
		for q.active >= q.MaxParallel {
			q.activeLock.Wait()
		}

		// Don't start new downloads on a failed queue.
		if q.err != nil {
			q.activeLock.L.Unlock()
			q.done = true
			return eris.Wrapf(q.err, "one or more downloads failed\n%s", api.Stacktrace(2))
		}

		q.active++
		q.activeLock.L.Unlock()

		go q.download(ctx, item, q.progress[idx])
	}

	// Wait for all running downloads to finish.
	q.activeLock.L.Lock()
	for q.active > 0 {
		q.activeLock.Wait()
	}
	q.activeLock.L.Unlock()

	q.done = true
	// Wake NextResult() in case it's waiting for further results.
	q.finishedLock.Broadcast()

	if q.err != nil {
		return eris.Wrapf(q.err, "one or more downloads failed\n%s", api.Stacktrace(2))
	}

	return nil
}

func (q *Queue) handleError(err error) {
	q.activeLock.L.Lock()
	defer q.activeLock.L.Unlock()

	if q.err == nil {
		// Don't overwrite an existing error. This means we end up silencing
		// this error but that probably won't matter since the queue already
		// failed.
		q.err = err
	}

	// Notify Run() that the queue failed
	q.active--
	q.activeLock.Broadcast()

	// Notify NextResult() that the queue failed
	q.finishedLock.Broadcast()
}

func (q *Queue) download(ctx context.Context, item *QueueItem, progress *uint32) {
	defer api.CrashReporter(ctx)

	// TODO: Implement a better algorithm for URL ordering, preferably by
	// ranking based on mirror preference but with some randomness so we don't
	// send all requests to the same mirror...
	// For now we just shuffle the URLs randomly to evenly distribute the load
	// across mirrors.
	urls := make([]string, len(item.Mirrors))
	copy(urls, item.Mirrors)
	rand.Shuffle(len(urls), sort.StringSlice(urls).Swap)

	f, err := os.Create(item.Filepath)
	if err != nil {
		q.handleError(eris.Wrapf(err, "failed to create %s", item.Filepath))
		return
	}
	defer f.Close()

	var hasher hash.Hash
	if item.Checksum != nil {
		hasher = sha256.New()
	}

	var lastError error
	midx := -1
	success := false
	buffer := make([]byte, 4*1024)
	filesize := item.Filesize
	for try := 0; try < q.Retries; try++ {
		if q.err != nil {
			return
		}

		midx++
		if midx >= len(urls) {
			midx = 0
		}

		req, err := http.NewRequest("GET", urls[midx], nil)
		if err != nil {
			q.handleError(eris.Wrapf(err, "failed to build request for %s", urls[midx]))
			return
		}

		req.Header.Set("User-Agent", userAgent)
		if *progress > 0 {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-", *progress))
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
				q.handleError(eris.Wrapf(err, "failed to seek in %s", item.Filepath))
				resp.Body.Close()
				return
			}

			atomic.StoreUint32(progress, 0)
			if hasher != nil {
				hasher.Reset()
			}
		}

		if item.Filesize > 0 && resp.ContentLength != item.Filesize {
			if resp.StatusCode == 200 || (resp.ContentLength+int64(*progress)) != item.Filesize {
				api.Log(ctx, api.LogWarn, "%s has unexpected size %d != %d", urls[midx], resp.ContentLength, item.Filesize)

				if item.Checksum != nil {
					// We still have a checksum to verify that the contents are fine so let's assume that this difference is fine.
					filesize = resp.ContentLength + int64(*progress)
				}
			}
		}

		for {
			read, err := resp.Body.Read(buffer)
			// Read() can return io.EOF for the last available block.
			// This means that we have to process the received data before we can take a look at the error.
			if read > 0 {
				_, err := f.Write(buffer[:read])
				if err != nil {
					q.handleError(eris.Wrap(err, "failed to write"))
					resp.Body.Close()
					return
				}

				atomic.AddUint32(progress, uint32(read))
				atomic.AddUint32(&q.periodReceivedBytes, uint32(read))

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

			if ctx.Err() != nil || q.err != nil {
				resp.Body.Close()
				return
			}
		}

		resp.Body.Close()

		if success {
			break
		}
	}

	if !success {
		q.handleError(lastError)
		return
	}

	pos, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		q.handleError(eris.Wrap(err, "failed to check file position"))
		return
	}

	if pos != int64(*progress) {
		q.handleError(eris.Errorf("internal consistency error for %s: file position is %d but received %d bytes", urls[midx], pos, *progress))
		return
	}

	if filesize > 0 && pos != filesize {
		q.handleError(eris.Errorf("%s was %d bytes but expected %d", urls[midx], pos, filesize))
		return
	}

	if hasher != nil {
		fileSum := hasher.Sum(nil)
		if !bytes.Equal(fileSum, item.Checksum) {
			q.handleError(eris.Errorf("%s failed due to a checksum mismatch (%s != %s)", urls[midx], fileSum, item.Checksum))
			return
		}
	}

	// Let Run() know that it can launch the next download.
	q.activeLock.L.Lock()
	q.active--
	q.activeLock.Signal()
	q.activeLock.L.Unlock()

	// Send the result to NextResult().
	q.finishedLock.L.Lock()
	q.finishedItems = append(q.finishedItems, item)
	q.finishedLock.Broadcast()
	q.finishedLock.L.Unlock()
}

func (q *Queue) Error() error {
	return eris.Wrap(q.err, "one or more downloads failed")
}

func (q *Queue) NextResult() bool {
	for {
		if q.err != nil {
			return false
		}

		q.finishedLock.L.Lock()
		if len(q.finishedItems) > 0 {
			q.result = q.finishedItems[0]
			q.finishedItems = q.finishedItems[1:]
			q.finishedLock.L.Unlock()
			return true
		}

		if q.done {
			q.finishedLock.L.Unlock()
			return false
		}

		q.finishedLock.Wait()
		q.finishedLock.L.Unlock()
	}
}

func (q *Queue) Result() *QueueItem {
	return q.result
}

func (q *Queue) updateProgressTicker() {
	for !q.done && q.err == nil {
		q.speedTracker.Track(int(atomic.SwapUint32(&q.periodReceivedBytes, 0)))

		totalReceived := uint32(0)
		for _, item := range q.progress {
			totalReceived += atomic.LoadUint32(item)
		}

		progress := float32(totalReceived) / float32(q.totalBytes)
		q.ProgressCb(progress, q.speedTracker.GetSpeed())

		time.Sleep(300 * time.Millisecond)
	}
}

func (q *Queue) Abort() {
	q.err = eris.New("aborted")

	// Notify Run() that the queue failed
	q.activeLock.Broadcast()

	// Notify NextResult() that the queue failed
	q.finishedLock.Broadcast()
}
