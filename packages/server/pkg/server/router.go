package server

import (
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/unrolled/secure"

	"github.com/ngld/knossos/packages/api/api"
	"github.com/ngld/knossos/packages/server/pkg/auth"
	"github.com/ngld/knossos/packages/server/pkg/config"
	"github.com/ngld/knossos/packages/server/pkg/db/queries"
	"github.com/ngld/knossos/packages/server/pkg/nblog"
)

func corsMiddleware(next http.Handler, origins []string) http.Handler {
	sort.Strings(origins)

	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		reqOrigin := strings.ToLower(r.Header.Get("origin"))

		if r.Method == "OPTIONS" || reqOrigin != "" {
			idx := sort.SearchStrings(origins, reqOrigin)
			if idx >= len(origins) || origins[idx] != reqOrigin {
				rw.WriteHeader(403)
				return
			}
		}

		if reqOrigin != "" {
			rw.Header().Set("Access-Control-Allow-Origin", reqOrigin)
			rw.Header().Set("Access-Control-Allow-Headers", "content-type")
		}

		if r.Method == "OPTIONS" {
			rw.WriteHeader(200)
			return
		}

		next.ServeHTTP(rw, r)
	})
}

func startMux(pool *pgxpool.Pool, q *queries.DBQuerier, cfg *config.Config) error {
	server := api.NewNebulaServer(nebula{
		Pool: pool,
		Q:    q,
		Cfg:  cfg,
	})

	staticRoot, err := filepath.Abs(cfg.HTTP.StaticRoot)
	if err != nil {
		return err
	}
	staticFS := http.Dir(staticRoot)

	r := mux.NewRouter()
	r.PathPrefix(server.PathPrefix()).Handler(corsMiddleware(server, []string{"https://files.client.fsnebula.org", "http://localhost:8080"}))
	r.PathPrefix("/js/").Handler(http.FileServer(staticFS))
	r.PathPrefix("/css/").Handler(http.FileServer(staticFS))
	r.HandleFunc("/", func(rw http.ResponseWriter, r *http.Request) {
		f, err := staticFS.Open("index.html")
		if err != nil {
			rw.WriteHeader(500)
			return
		}
		defer f.Close()

		http.ServeContent(rw, r, "index.html", time.Now(), f)
	})

	sm := secure.New(secure.Options{
		// TODO: Figure out how to only enable in production
		// SSLRedirect: true,
		IsDevelopment:      true,
		BrowserXssFilter:   true,
		ContentTypeNosniff: true,
		FrameDeny:          true,
	})

	muxServer := http.Server{
		Handler:      sm.Handler(auth.MakeAuthMiddleware(nblog.MakeLogMiddleware(r))),
		Addr:         cfg.HTTP.Address,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	return muxServer.ListenAndServe()
}
