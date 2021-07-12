package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/ngld/knossos/packages/updater/downloader"
	"github.com/rotisserie/eris"
)

type configRootFS struct {
	Type    string   `json:"type"`
	DiffIDs []string `json:"diff_ids"`
}

type config struct {
	Architecture string       `json:"architecture"`
	OS           string       `json:"os"`
	RootFS       configRootFS `json:"rootfs"`
}

var client = http.Client{
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

func uploadLayer(f io.ReadSeeker, name, token string) (string, error) {
	hasher := sha256.New()
	_, err := io.Copy(hasher, f)
	if err != nil {
		return "", eris.Wrapf(err, "failed to hash %s", name)
	}

	_, err = f.Seek(0, io.SeekStart)
	if err != nil {
		return "", eris.Wrap(err, "failed to seek")
	}
	digest := "sha256:" + hex.EncodeToString(hasher.Sum(nil))

	req, err := http.NewRequest("HEAD", downloader.RepoEndpoint+downloader.Repo+"/blobs/"+digest, nil)
	if err != nil {
		return "", eris.Wrap(err, "failed to construct layer check request")
	}

	req.Header.Add("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		return "", eris.Wrap(err, "layer check request failed")
	}
	resp.Body.Close()

	if resp.StatusCode != 200 {
		// layer hasn't been uploaded, yet
		req, err = http.NewRequest("POST", downloader.RepoEndpoint+downloader.Repo+"/blobs/uploads/", nil)
		if err != nil {
			return "", eris.Wrap(err, "failed to construct upload init request")
		}

		req.Header.Add("Authorization", "Bearer "+token)
		resp, err := client.Do(req)
		if err != nil {
			return "", eris.Wrap(err, "upload init request failed")
		}
		resp.Body.Close()

		if resp.StatusCode != 202 {
			return "", eris.Errorf("upload init request failed with status code %d", resp.StatusCode)
		}

		endpointURL, err := url.Parse(downloader.RepoEndpoint)
		if err != nil {
			return "", eris.Wrapf(err, "failed to parse repo endpoint %s", downloader.RepoEndpoint)
		}

		destURL, err := endpointURL.Parse(resp.Header.Get("Location"))
		if err != nil {
			return "", eris.Wrapf(err, "failed to parse upload destination %s", resp.Header.Get("Location"))
		}

		query := destURL.Query()
		query.Add("digest", digest)
		destURL.RawQuery = query.Encode()
		uploadDest := destURL.String()

		req, err = http.NewRequest("PUT", uploadDest, f)
		if err != nil {
			return "", eris.Wrap(err, "failed to construct upload request")
		}

		req.Header.Add("Authorization", "Bearer "+token)
		req.Header.Add("Content-Type", "application/octet-stream")
		resp, err = client.Do(req)
		if err != nil {
			return "", eris.Wrap(err, "failed upload request")
		}
		resp.Body.Close()

		if resp.StatusCode != 201 {
			return "", eris.Errorf("upload failed with status %d", resp.StatusCode)
		}
	}

	return digest, nil
}

func uploadArchive(filename, tag string) error {
	req, err := http.NewRequest("GET", "https://ghcr.io/token?scope=repository:ngld/knossos/releases:push&service=ghcr.io", nil)
	if err != nil {
		return eris.Wrap(err, "failed to request token")
	}

	basicAuth := "user:" + os.Getenv("GITHUB_TOKEN")
	basicAuth = base64.RawStdEncoding.EncodeToString([]byte(basicAuth))
	req.Header.Add("Authorization", "Basic "+basicAuth)
	resp, err := client.Do(req)
	if err != nil {
		return eris.Wrap(err, "failed to request token")
	}

	if resp.StatusCode != 200 {
		resp.Body.Close()
		return eris.Errorf("token request failed with status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		resp.Body.Close()
		return eris.Wrap(err, "failed to read token response")
	}
	resp.Body.Close()

	var decodedResp downloader.TokenResponse
	err = json.Unmarshal(data, &decodedResp)
	if err != nil {
		return eris.Wrap(err, "failed to decode token response")
	}
	token := decodedResp.Token

	f, err := os.Open(filename)
	if err != nil {
		return eris.Wrapf(err, "failed to open %s", filename)
	}
	defer f.Close()

	fileSize, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		return eris.Wrap(err, "failed to determine file size")
	}

	_, err = f.Seek(0, io.SeekStart)
	if err != nil {
		return eris.Wrap(err, "failed to reset file position")
	}

	digest, err := uploadLayer(f, filename, token)
	if err != nil {
		return eris.Wrap(err, "failed to upload archive")
	}

	config := config{
		Architecture: "amd64",
		OS:           "linux",
		RootFS: configRootFS{
			Type:    "layers",
			DiffIDs: []string{digest},
		},
	}
	configStr, err := json.Marshal(config)
	if err != nil {
		return eris.Wrap(err, "failed to encode config")
	}

	configDigest, err := uploadLayer(bytes.NewReader(configStr), "config", token)
	if err != nil {
		return eris.Wrap(err, "failed to upload config")
	}

	manifest := downloader.RegistryManifest{
		SchemaVersion: 2,
		MediaType:     "application/vnd.docker.distribution.manifest.v2+json",
		Config: downloader.Blob{
			MediaType: "application/vnd.docker.container.image.v1+json",
			Size:      len(configStr),
			Digest:    configDigest,
		},
		Layers: []downloader.Blob{
			{
				MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
				Size:      int(fileSize),
				Digest:    digest,
			},
		},
	}

	encodedManifest, err := json.Marshal(manifest)
	if err != nil {
		return eris.Wrap(err, "failed to encode manifest")
	}

	req, err = http.NewRequest("PUT", downloader.RepoEndpoint+downloader.Repo+"/manifests/"+tag, bytes.NewBuffer(encodedManifest))
	if err != nil {
		return eris.Wrap(err, "failed to construct manifest request")
	}

	req.Header.Add("Authorization", "Bearer "+token)
	req.Header.Add("Content-Type", manifest.MediaType)
	resp, err = client.Do(req)
	if err != nil {
		return eris.Wrap(err, "failed to upload manifest")
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return eris.Wrapf(err, "failed to read error for code %d after manifest upload", resp.StatusCode)
		}

		return eris.Errorf("manifest upload failed with code %d: %s", resp.StatusCode, data)
	}

	return nil
}
