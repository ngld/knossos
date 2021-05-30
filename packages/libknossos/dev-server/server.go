package main

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/rotisserie/eris"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/proto"

	"github.com/ngld/knossos/packages/api/client"
	"github.com/ngld/knossos/packages/libknossos/pkg/api"
	"github.com/ngld/knossos/packages/libknossos/pkg/storage"
	"github.com/ngld/knossos/packages/libknossos/pkg/twirp"
)

var (
	wsLock        = sync.Mutex{}
	wsConnections = make([]*websocket.Conn, 0)
	upgrader      = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return strings.HasPrefix(r.Header.Get("Origin"), "http://localhost")
		},
	}
)

func main() {
	log.Logger = log.Output(getConsoleWriter(os.Stdout))
	zerolog.ErrorStackMarshaler = func(err error) interface{} {
		return eris.ToString(err, true)
	}

	log.Logger = log.Logger.With().Caller().Stack().Logger()

	profilePath := ""
	if runtime.GOOS == "windows" {
		profilePath = filepath.Join(os.Getenv("AppData"), "Knossos")
	}

	if profilePath == "" {
		log.Fatal().Msg("Failed to determine profile path")
	}

	log.Info().Msg("Initialising libknossos")
	ctx := context.Background()

	knParams := api.KnossosCtxParams{
		LogCallback:     logCallback,
		MessageCallback: msgDispatcher,
		SettingsPath:    profilePath,
		ResourcePath:    "/does/not/exist",
	}
	ctx = api.WithKnossosContext(ctx, knParams)
	err := storage.Open(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to open database")
	}

	log.Info().Msg("Starting server")

	twirpServer, err := twirp.NewServer()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to start server")
	}

	twirpPrefix := twirpServer.(client.TwirpServer).PathPrefix()

	router := mux.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			rw.Header().Set("Access-Control-Allow-Origin", "*")
			rw.Header().Set("Access-Control-Allow-Headers", "content-type")

			if r.Method == "OPTIONS" {
				rw.WriteHeader(200)
				return
			}

			ctx := api.WithKnossosContext(r.Context(), knParams)
			next.ServeHTTP(rw, r.WithContext(ctx))
		})
	})

	router.PathPrefix(twirpPrefix).Handler(twirpServer)
	router.PathPrefix("/ws").HandlerFunc(wsHandler)
	router.PathPrefix("/ref").HandlerFunc(refHandler)

	muxServer := http.Server{
		Handler:      router,
		Addr:         "localhost:8100",
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	err = muxServer.ListenAndServe()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to listen")
	}
}

func wsHandler(rw http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(rw, r, nil)
	if err != nil {
		log.Error().Err(err).Msg("Failed to accept incoming WS connection")
		rw.WriteHeader(500)
		return
	}

	go func() {
		for {
			_, _, err := conn.NextReader()
			if err != nil {
				conn.Close()

				wsLock.Lock()
				for idx, c := range wsConnections {
					if c == conn {
						wsConnections = append(wsConnections[0:idx], wsConnections[idx+1:]...)
						break
					}
				}
				wsLock.Unlock()
				break
			}
		}
	}()

	wsLock.Lock()
	wsConnections = append(wsConnections, conn)
	wsLock.Unlock()
}

func refHandler(rw http.ResponseWriter, r *http.Request) {
	// /ref/
	fileID := r.URL.Path[5:]
	fileRef, err := storage.GetFile(r.Context(), fileID)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to look up file ref %s", fileID)
		rw.WriteHeader(500)
		return
	}

	localPath := ""
	for _, item := range fileRef.Urls {
		if strings.HasPrefix(item, "file://") {
			localPath = filepath.FromSlash(item[7:])
			break
		}
	}

	if localPath == "" {
		rw.WriteHeader(404)
		return
	}

	f, err := os.Open(localPath)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to open %s", localPath)
		rw.WriteHeader(500)
		return
	}

	_, err = io.Copy(rw, f)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to read %s", localPath)
		rw.WriteHeader(500)
		return
	}
}

func msgDispatcher(cse *client.ClientSentEvent) error {
	encoded, err := proto.Marshal(cse)
	if err != nil {
		return err
	}

	wsLock.Lock()
	defer wsLock.Unlock()

	for _, conn := range wsConnections {
		err = conn.WriteMessage(websocket.BinaryMessage, encoded)
		if err != nil {
			log.Error().Err(err).Msg("Failed to send on a WS connection")
		}
	}

	return nil
}
