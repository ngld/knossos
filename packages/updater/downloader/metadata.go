package downloader

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
	resp, err := client.Get(RepoEndpoint + Repo + "/blobs/" + digest)
	if err != nil {
		return "", err
	}

	return resp.Header.Get("Location"), nil
}
