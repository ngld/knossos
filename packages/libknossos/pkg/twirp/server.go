package twirp

import (
	"context"
	"net/http"
	"runtime"

	"github.com/ngld/knossos/packages/api/client"
	"github.com/ngld/knossos/packages/libknossos/pkg/api"
	"github.com/ngld/knossos/packages/libknossos/pkg/storage"
	"github.com/rotisserie/eris"
	"github.com/twitchtv/twirp"
)

type knossosServer struct {
	client.Knossos
}

type wrappedError interface {
	Unwrap() error
}

func NewServer() (http.Handler, error) {
	return client.NewKnossosServer(&knossosServer{}, twirp.WithServerHooks(&twirp.ServerHooks{
		Error: func(c context.Context, twErr twirp.Error) context.Context {
			err := twErr.(error)
			// Unwrap the error, if possible, to get a proper stack trace.
			if wrapped, ok := twErr.(wrappedError); ok {
				err = wrapped.Unwrap()
			}
			api.Log(c, api.LogError, "Twirp error: %s", eris.ToString(err, true))
			return c
		},
	})), nil
}

func (kn *knossosServer) Wakeup(context.Context, *client.NullMessage) (*client.WakeupResponse, error) {
	return &client.WakeupResponse{
		Success: true,
		Version: "0.0.0",
		Os:      runtime.GOOS,
	}, nil
}

func (kn *knossosServer) GetSettings(ctx context.Context, _ *client.NullMessage) (*client.Settings, error) {
	return storage.GetSettings(ctx)
}

func (kn *knossosServer) SaveSettings(ctx context.Context, settings *client.Settings) (*client.SuccessResponse, error) {
	err := storage.SaveSettings(ctx, settings)
	if err != nil {
		return nil, err
	}

	return &client.SuccessResponse{Success: true}, nil
}

func (kn *knossosServer) GetVersion(ctx context.Context, _ *client.NullMessage) (*client.VersionResult, error) {
	return &client.VersionResult{
		Version: api.Version,
		Commit:  api.Commit,
	}, nil
}
