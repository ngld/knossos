package twirp

import (
	"context"
	"net/http"
	"runtime"

	"github.com/ngld/knossos/packages/api/client"
	"github.com/ngld/knossos/packages/libknossos/pkg/storage"
)

type knossosServer struct {
	client.Knossos
}

func NewServer() (http.Handler, error) {
	return client.NewKnossosServer(&knossosServer{}), nil
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
