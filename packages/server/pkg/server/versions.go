package server

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/ngld/knossos/packages/api/api"
	"github.com/ngld/knossos/packages/server/pkg/nblog"
	"github.com/twitchtv/twirp"
)

type versionUpdateInput struct {
	Key     string `json:"key"`
	Version string `json:"version"`
}

func (neb nebula) GetVersions(ctx context.Context, req *api.NullRequest) (*api.VersionsResponse, error) {
	rows, err := neb.Q.GetVersions(ctx)
	if err != nil {
		nblog.Log(ctx).Error().Err(err).Msg("Failed to fetch versions")
		return nil, twirp.InternalError("internal error")
	}

	resp := &api.VersionsResponse{
		Versions: make(map[string]string),
	}

	for _, row := range rows {
		resp.Versions[*row.Key] = *row.Version
	}

	return resp, nil
}

func registerVersionRoutes(neb nebula, router *mux.Router) {
	router.PathPrefix("/rest/update_version").HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		authToken := r.Header.Get("Authorization")
		if !strings.HasPrefix(authToken, "Bearer ") {
			rw.WriteHeader(403)
			return
		}

		if subtle.ConstantTimeCompare([]byte(authToken[7:]), []byte(neb.Cfg.Keys.VersionUpdateKey)) == 0 {
			rw.WriteHeader(403)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			nblog.Log(r.Context()).Error().Err(err).Msg("Failed to read body")
			rw.WriteHeader(400)
			return
		}

		var input versionUpdateInput
		err = json.Unmarshal(body, &input)
		if err != nil {
			nblog.Log(r.Context()).Error().Err(err).Msg("Failed to parse body")
			rw.WriteHeader(400)
			return
		}

		_, err = neb.Q.UpdateVersion(r.Context(), input.Key, input.Version)
		if err != nil {
			nblog.Log(r.Context()).Error().Err(err).Msg("Failed to store version")
			rw.WriteHeader(500)
			return
		}

		rw.WriteHeader(200)
	})
}
