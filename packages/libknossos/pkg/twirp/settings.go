package twirp

import (
	"context"

	"github.com/ngld/knossos/packages/api/client"
	"github.com/ngld/knossos/packages/libknossos/pkg/fso_interop"
)

func (kn *knossosServer) GetFSOSettings(ctx context.Context, req *client.NullMessage) (*client.FSOSettings, error) {
	settings, err := fso_interop.LoadSettings(ctx)
	if err != nil {
		return nil, err
	}

	return settings, nil
}

func (kn *knossosServer) SaveFSOSettings(ctx context.Context, req *client.FSOSettings) (*client.SuccessResponse, error) {
	err := fso_interop.SaveSettings(ctx, req)
	if err != nil {
		return nil, err
	}

	return &client.SuccessResponse{Success: true}, nil
}
