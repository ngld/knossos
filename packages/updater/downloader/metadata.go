package downloader

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	repoEndpoint  = "https://ghcr.io/v2/"
	tokenEndpoint = "https://ghcr.io/token?scope=repository:%s&service=ghcr.io"
	repo          = "ngld/knossos/nebula"
)

type tokenResponse struct {
	token string
}

type listResponse struct {
	name string
	tags []string
}

type blob struct {
	mediaType string
	size      int
	digest    string
}

type manifestResponse struct {
	schemaVersion int
	mediaType     string
	config        blob
	layers        []blob
}

var client = http.Client{
	Timeout: 3 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

func getToken(repo string) (string, error) {
	resp, err := client.Get(fmt.Sprintf(tokenEndpoint, repo))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var result tokenResponse
	err = json.Unmarshal(body, &result)
	if err != nil {
		return "", err
	}

	return result.token, nil
}

func doAuthenticatedRequest(ctx context.Context, path string, token string, dest interface{}) error {
	req, err := http.NewRequest("GET", repoEndpoint+path, nil)
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

func getAvailableVersions(ctx context.Context, token string) ([]string, error) {
	var response listResponse
	err := doAuthenticatedRequest(ctx, repo+"/tags/list", token, &response)
	if err != nil {
		return nil, err
	}

	return response.tags, nil
}

func getBlobURL(ctx context.Context, token string, digest string) (string, error) {
	resp, err := client.Get(repoEndpoint + repo + "/blobs/" + digest)
	if err != nil {
		return "", err
	}

	return resp.Header.Get("Location"), nil
}

func downloadVersion(ctx context.Context, token string, version string) error {
}
