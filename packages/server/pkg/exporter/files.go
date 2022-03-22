package exporter

import (
	"context"

	"github.com/ngld/knossos/packages/server/pkg/db/queries"
	"github.com/ngld/knossos/packages/server/pkg/nblog"
	"github.com/rotisserie/eris"
)

func GetFileURLs(ctx context.Context, q queries.Querier, fid int) ([]string, error) {
	data, err := q.GetPublicFileByID(ctx, int32(fid))
	if err != nil {
		return []string{}, eris.Wrapf(err, "failed to fetch file %d", fid)
	}

	return GetFileURLsFromValues(ctx, fid, data.StorageKey, data.External)
}

func GetFileURLsFromValues(ctx context.Context, fid int, storageKey string, external []string) ([]string, error) {
	if storageKey != "" {
		if len(external) > 0 {
			return external, nil
		}
		nblog.Log(ctx).Warn().Msgf("Generating teaser URLs is not yet supported (%s)", storageKey)
		return []string{}, nil
	}
	return []string{}, nil
}
