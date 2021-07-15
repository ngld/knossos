package downloader

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/rotisserie/eris"
)

const (
	RepoEndpoint  = "https://ghcr.io/v2/"
	tokenEndpoint = "https://ghcr.io/token?scope=repository:%s:pull&service=ghcr.io"
	Repo          = "ngld/knossos/releases"
)

type TokenResponse struct {
	Token  string          `json:"token"`
	Errors []responseError `json:"errors"`
}

type responseError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type listResponse struct {
	Name   string          `json:"name"`
	Tags   []string        `json:"tags"`
	Errors []responseError `json:"errors"`
}

type Blob struct {
	MediaType string `json:"mediaType"`
	Digest    string `json:"digest"`
	Size      int    `json:"size"`
}

type RegistryManifest struct {
	MediaType     string `json:"mediaType"`
	Config        Blob   `json:"config"`
	Layers        []Blob `json:"layers"`
	SchemaVersion int    `json:"schemaVersion"`
}

var client = http.Client{
	Timeout: 3 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

func GetToken(repo string) (string, error) {
	resp, err := client.Get(fmt.Sprintf(tokenEndpoint, repo))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var result TokenResponse
	err = json.Unmarshal(body, &result)
	if err != nil {
		return "", err
	}

	if len(result.Errors) > 0 {
		return "", eris.Errorf("GitHub token error: %s", result.Errors[0].Message)
	}
	return result.Token, nil
}

func doAuthenticatedRequest(ctx context.Context, path string, token string, dest interface{}) error {
	req, err := http.NewRequest("GET", RepoEndpoint+path, nil)
	if err != nil {
		return err
	}
	req.Header.Add("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	return json.Unmarshal(body, dest)
}

func GetAvailableVersions(ctx context.Context, token string) ([]string, error) {
	var response listResponse
	err := doAuthenticatedRequest(ctx, Repo+"/tags/list", token, &response)
	if err != nil {
		return nil, err
	}

	if len(response.Errors) > 0 {
		return nil, eris.Errorf("GitHub error: %s", response.Errors[0].Message)
	}
	return response.Tags, nil
}

func GetBlobURL(ctx context.Context, token string, digest string) (string, error) {
	req, err := http.NewRequest("GET", RepoEndpoint+Repo+"/blobs/"+digest, nil)
	if err != nil {
		return "", err
	}
	req.Header.Add("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	resp.Body.Close()

	if resp.StatusCode != 307 {
		return "", eris.Errorf("unexpected status %d", resp.StatusCode)
	}
	return resp.Header.Get("Location"), nil
}

func DownloadVersion(ctx context.Context, token, tag, dest string, progressCb func(float32, string)) error {
	var response RegistryManifest
	err := doAuthenticatedRequest(ctx, Repo+"/manifests/"+tag, token, &response)
	if err != nil {
		return err
	}

	if len(response.Layers) < 1 {
		return eris.New("no layers found")
	}

	if response.Layers[0].MediaType != "application/vnd.docker.image.rootfs.diff.tar.gzip" {
		return eris.Errorf("the layer has an unexpected type: %s", response.Layers[0].MediaType)
	}

	url, err := GetBlobURL(ctx, token, response.Layers[0].Digest)
	if err != nil {
		return eris.Wrap(err, "failed to generate blob URL")
	}

	f, err := os.Create(dest)
	if err != nil {
		return eris.Wrap(err, "failed to open destination file")
	}
	defer f.Close()

	resp, err := http.Get(url)
	if err != nil {
		return eris.Wrap(err, "failed to download")
	}
	defer resp.Body.Close()

	done := false
	defer func() {
		done = true
	}()

	go func() {
		for !done {
			pos, err := f.Seek(0, io.SeekCurrent)
			if err == nil {
				progressCb(float32(pos)/float32(resp.ContentLength), "Downloading")
			}

			time.Sleep(500 * time.Millisecond)
		}
	}()

	_, err = io.Copy(f, resp.Body)
	if err != nil {
		return eris.Wrap(err, "download was interrupted")
	}

	// TODO verify checksum (digest)
	return nil
}
