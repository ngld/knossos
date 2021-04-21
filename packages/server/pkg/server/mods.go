package server

import (
	"context"
	"encoding/hex"

	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4"
	"github.com/ngld/knossos/packages/api/api"
	"github.com/ngld/knossos/packages/server/pkg/db/queries"
	"github.com/ngld/knossos/packages/server/pkg/nblog"
	"github.com/rotisserie/eris"
	"github.com/twitchtv/twirp"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func GetFileURL(ctx context.Context, q *queries.DBQuerier, fid int) (string, error) {
	data, err := q.GetPublicFileByID(ctx, int32(fid))
	if err != nil {
		return "", eris.Wrapf(err, "failed to fetch file %d", fid)
	}

	if data.StorageKey.Status == pgtype.Present {
		if data.External.Status == pgtype.Present {
			return data.External.Elements[0].String, nil
		} else {
			nblog.Log(ctx).Warn().Msgf("Generating teaser URLs is not yet supported (%s)", data.StorageKey.String)
			return "", nil
		}
	}
	return "", nil
}

func (neb nebula) GetModList(ctx context.Context, req *api.ModListRequest) (*api.ModListResponse, error) {
	limit := int(req.Limit)
	if limit > 300 {
		limit = 300
	}

	query := `SELECT m.modid, m.title, m.type, max(r.version), COUNT(r.*) AS release_count,
		max(f.storage_key) AS storage_key, max(f.external) AS external
		FROM mods AS m
		LEFT JOIN (SELECT mod_aid, MAX(id) AS id FROM mod_releases WHERE private = false GROUP BY mod_aid) AS rm ON rm.mod_aid = m.aid
		LEFT JOIN mod_releases AS r ON r.id = rm.id
		LEFT OUTER JOIN files AS f ON f.id = r.teaser
		WHERE m.private = false`

	if req.Query != "" {
		query += " AND m.normalized_title LIKE '%' || normalize_string($3) || '%'"
	}

	query += " GROUP BY m.aid"

	if req.Sort == api.ModListRequest_NAME {
		query += " ORDER BY title"
	}

	query += " LIMIT $1 OFFSET $2"

	args := []interface{}{limit, req.Offset}
	if req.Query != "" {
		args = append(args, req.Query)
	}

	modCount, err := neb.Q.GetPublicModCount(ctx)
	if err != nil {
		nblog.Log(ctx).Error().Err(err).Msg("Failed to fetch public mod count")
		return nil, twirp.InternalError("internal error")
	}

	result, err := neb.Pool.Query(ctx, query, args...)
	if err != nil {
		nblog.Log(ctx).Error().Err(err).Msg("Failed to fetch public mod list")
		return nil, twirp.InternalError("internal error")
	}
	defer result.Close()

	modItems := make([]*api.ModListItem, 0)
	for result.Next() {
		row := new(api.ModListItem)
		var storageKey pgtype.Text
		var modType pgtype.Int2
		var external pgtype.TextArray

		err = result.Scan(&row.Modid, &row.Title, &modType, &row.Version, &row.ReleaseCount, &storageKey, &external)
		if err != nil {
			nblog.Log(ctx).Error().Err(err).Msg("Failed to read mod row")
			continue
		}

		teaserURL := ""
		if storageKey.Status == pgtype.Present {
			if external.Status == pgtype.Present {
				teaserURL = external.Elements[0].String
			} else {
				nblog.Log(ctx).Warn().Msgf("Generating teaser URLs is not yet supported (%s)", storageKey.String)
			}
		}

		row.Teaser = teaserURL
		modItems = append(modItems, row)
	}

	return &api.ModListResponse{
		Count: int32(modCount.Int),
		Mods:  modItems,
	}, nil
}

// GetModDetails retrieves details for the given mod and returns them
func (neb nebula) GetModDetails(ctx context.Context, req *api.ModDetailsRequest) (*api.ModDetailsResponse, error) {
	if req.Modid == "" {
		return nil, twirp.RequiredArgumentError("Modid")
	}
	if req.Version == "" && !req.Latest {
		return nil, twirp.RequiredArgumentError("Version")
	}

	if req.Latest {
		version, err := neb.Q.GetLatestPublicModVersion(ctx, req.Modid)
		if err != nil {
			if eris.Is(err, pgx.ErrNoRows) {
				return nil, twirp.NotFoundError("no such mod")
			}

			nblog.Log(ctx).Error().Err(err).Msgf("Failed to determine latest version for mod %s", req.Modid)
			return nil, twirp.InternalError("internal error")
		}

		req.Version = version.Version.String
	}

	details, err := neb.Q.GetPublicReleaseByModVersion(ctx, req.Modid, req.Version)
	if err != nil {
		if eris.Is(err, pgx.ErrNoRows) {
			return nil, twirp.NotFoundError("no such mod")
		}

		nblog.Log(ctx).Error().Err(err).Msgf("Failed to fetch data for public release %s (%s)", req.Modid, req.Version)
		return nil, twirp.InternalError("internal error")
	}

	bannerURL, err := GetFileURL(ctx, neb.Q, int(details.Banner.Int))
	if err != nil {
		return nil, twirp.InternalError("internal error")
	}

	screenshotURLs := make([]string, len(details.Screenshots.Elements))
	for idx, fid := range details.Screenshots.Elements {
		screenshotURLs[idx], err = GetFileURL(ctx, neb.Q, int(fid.Int))
		if err != nil {
			return nil, twirp.InternalError("internal error")
		}
	}

	videos := make([]string, len(details.Videos.Elements))
	for idx, video := range details.Videos.Elements {
		videos[idx] = video.String
	}

	result := &api.ModDetailsResponse{
		Title:         details.Title.String,
		Version:       details.Version.String,
		Type:          uint32(details.Type.Int),
		Stability:     uint32(details.Stability.Int),
		Description:   details.Description.String,
		Banner:        bannerURL,
		ReleaseThread: details.ReleaseThread.String,
		Screenshots:   screenshotURLs,
		Videos:        videos,
		Released:      &timestamppb.Timestamp{Seconds: details.Released.Time.Unix()},
		Updated:       &timestamppb.Timestamp{Seconds: details.Updated.Time.Unix()},
	}

	result.Versions, err = neb.Q.GetPublicModVersions(ctx, details.Aid.Int)
	if err != nil {
		nblog.Log(ctx).Error().Err(err).Msgf("Failed to fetch version list for mod %d", details.Aid.Int)
		return nil, twirp.InternalError("internal error")
	}

	if req.RequestDownloads {
		dlInfos, err := neb.Q.GetPublicDownloadsByRID(ctx, details.ID.Int)
		if err != nil {
			nblog.Log(ctx).Error().Err(err).Msg("Failed to fetch download info")
		} else {
			packages := map[string]*[]*api.ModDownloadArchive{}
			result.Downloads = make([]*api.ModDownloadPackage, 0)

			for _, row := range dlInfos {
				archives, found := packages[row.Package.String]
				if !found {
					apiPkg := &api.ModDownloadPackage{
						Name:     row.Package.String,
						Notes:    row.PackageNotes.String,
						Archives: make([]*api.ModDownloadArchive, 0),
					}

					result.Downloads = append(result.Downloads, apiPkg)
					packages[row.Package.String] = &apiPkg.Archives
					archives = &apiPkg.Archives
				}

				archive := &api.ModDownloadArchive{
					Label:    row.Label.String,
					Checksum: hex.EncodeToString(row.ChecksumDigest.Bytes),
					Size:     uint32(row.Filesize.Int),
					// TODO support internal links
					Links: make([]string, len(row.External.Elements)),
				}

				for idx, link := range row.External.Elements {
					archive.Links[idx] = link.String
				}

				*archives = append(*archives, archive)
			}
		}
	}

	return result, nil
}
