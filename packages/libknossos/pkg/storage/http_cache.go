package storage

import (
	"context"
	"encoding/json"
	"time"

	"go.etcd.io/bbolt"
)

var httpCacheBucket = []byte("httpCache")

type HTTPCacheEntry struct {
	LastAccessed time.Time
	FetchDate    string
	ETag         string
}

func GetHTTPCacheEntryForURL(ctx context.Context, url string) (*HTTPCacheEntry, error) {
	var entry *HTTPCacheEntry
	err := update(ctx, func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(httpCacheBucket)
		encoded := bucket.Get([]byte(url))
		// No entry found; return nil
		if encoded == nil {
			return nil
		}

		entry = new(HTTPCacheEntry)
		err := json.Unmarshal(encoded, entry)
		if err != nil {
			return err
		}

		// Update the LastAccessed field
		encoded, err = json.Marshal(&HTTPCacheEntry{
			LastAccessed: time.Now(),
			FetchDate:    entry.FetchDate,
			ETag:         entry.ETag,
		})
		if err != nil {
			return err
		}

		return bucket.Put([]byte(url), encoded)
	})
	if err != nil {
		return nil, err
	}
	return entry, nil
}

func SetHTTPCacheEntryForURL(ctx context.Context, url string, entry *HTTPCacheEntry) error {
	return update(ctx, func(tx *bbolt.Tx) error {
		encoded, err := json.Marshal(entry)
		if err != nil {
			return err
		}

		return tx.Bucket(httpCacheBucket).Put([]byte(url), encoded)
	})
}
