package helpers

import (
	"context"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"time"

	"github.com/ngld/knossos/packages/libknossos/pkg/api"
	"github.com/ngld/knossos/packages/libknossos/pkg/storage"
	"golang.org/x/net/publicsuffix"
)

var (
	userAgent  string
	httpClient = http.Client{
		Timeout: 10 * time.Second,
	}
)

func Init(ctx context.Context) error {
	userAgent = fmt.Sprintf("Knossos %s (+https://fsnebula.org/knossos/)", api.Version)

	jar, err := cookiejar.New(&cookiejar.Options{
		PublicSuffixList: publicsuffix.List,
	})
	if err != nil {
		return err
	}

	httpClient.Jar = jar
	return nil
}

func CachedGet(ctx context.Context, url string) (*http.Response, error) {
	cacheEntry, err := storage.GetHTTPCacheEntryForURL(ctx, url)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	if cacheEntry != nil {
		req.Header.Set("If-None-Match", cacheEntry.ETag)
		req.Header.Set("If-Modified-Since", cacheEntry.FetchDate)
	}

	res, err := HTTPDo(ctx, req)
	if err != nil {
		return nil, err
	}

	err = storage.SetHTTPCacheEntryForURL(ctx, url, &storage.HTTPCacheEntry{
		ETag:      res.Header.Get("ETag"),
		FetchDate: res.Header.Get("Date"),
	})
	if err != nil {
		return nil, err
	}

	return res, nil
}

func HTTPDo(ctx context.Context, req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", userAgent)
	return httpClient.Do(req)
}
