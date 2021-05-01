package server

import (
	"context"
	"encoding/hex"

	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4"
	"github.com/rotisserie/eris"
	"github.com/twitchtv/twirp"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/ngld/knossos/packages/api/api"
	"github.com/ngld/knossos/packages/api/common"
	"github.com/ngld/knossos/packages/server/pkg/db"
	"github.com/ngld/knossos/packages/server/pkg/db/queries"
	"github.com/ngld/knossos/packages/server/pkg/mods"
	"github.com/ngld/knossos/packages/server/pkg/nblog"
)

func GetFileURLs(ctx context.Context, q *queries.DBQuerier, fid int) ([]string, error) {
	data, err := q.GetPublicFileByID(ctx, int32(fid))
	if err != nil {
		return []string{}, eris.Wrapf(err, "failed to fetch file %d", fid)
	}

	if data.StorageKey.Status == pgtype.Present {
		if data.External.Status == pgtype.Present {
			urls := make([]string, len(data.External.Elements))
			for idx, el := range data.External.Elements {
				urls[idx] = el.String
			}

			return urls, nil
		}
		nblog.Log(ctx).Warn().Msgf("Generating teaser URLs is not yet supported (%s)", data.StorageKey.String)
		return []string{}, nil
	}
	return []string{}, nil
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

	bannerURL, err := GetFileURLs(ctx, neb.Q, int(details.Banner.Int))
	if err != nil {
		return nil, twirp.InternalError("internal error")
	}

	screenshotURLs := make([]string, len(details.Screenshots.Elements))
	for idx, fid := range details.Screenshots.Elements {
		urls, err := GetFileURLs(ctx, neb.Q, int(fid.Int))
		if err != nil {
			return nil, twirp.InternalError("internal error")
		}

		screenshotURLs[idx] = urls[0]
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
		Banner:        bannerURL[0],
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

func (neb nebula) RequestModInstall(ctx context.Context, req *api.ModInstallRequest) (*api.ModInstallResponse, error) {
	if req.Modid == "" {
		return nil, twirp.RequiredArgumentError("Modid")
	}
	if req.Version == "" {
		return nil, twirp.RequiredArgumentError("Version")
	}

	depSnapshot, err := mods.GetDependencySnapshot(ctx, neb.Q, req.Modid, req.Version)
	if err != nil {
		return nil, eris.Wrap(err, "failed to resolve dependencies")
	}

	result := &api.ModInstallResponse{Releases: make([]*common.Release, 0, len(depSnapshot))}
	for modid, version := range depSnapshot {
		details, err := neb.Q.GetPublicReleaseByModVersion(ctx, modid, version)
		if err != nil {
			return nil, err
		}

		rel := &common.Release{
			Modid:         req.Modid,
			Version:       req.Version,
			Title:         details.Title.String,
			Folder:        req.Modid + "-" + req.Version,
			Description:   details.Description.String,
			ReleaseThread: details.ReleaseThread.String,
			Released:      timestamppb.New(details.Released.Time),
			Updated:       timestamppb.New(details.Updated.Time),
			Notes:         details.Notes.String,
		}

		switch db.EngineStability(details.Stability.Int) {
		case db.EngineStable:
			rel.Stability = common.ReleaseStability_STABLE
		case db.EngineRC:
			rel.Stability = common.ReleaseStability_RC
		case db.EngineNightly:
			rel.Stability = common.ReleaseStability_NIGHTLY
		case db.EngineUnknown:
		}

		switch db.ModType(details.Type.Int) {
		case db.TypeEngine:
			rel.Type = common.ModType_ENGINE
		case db.TypeExtension:
			rel.Type = common.ModType_EXTENSION
		case db.TypeMod:
			rel.Type = common.ModType_MOD
		case db.TypeTool:
			rel.Type = common.ModType_TOOL
		case db.TypeTotalConversion:
			rel.Type = common.ModType_TOTAL_CONVERSION
		}

		// TODO: Properly support TCs
		rel.Parent = "FS2"

		if details.Teaser.Status == pgtype.Present {
			urls, err := GetFileURLs(ctx, neb.Q, int(details.Teaser.Int))
			if err != nil {
				return nil, err
			}

			rel.Teaser = &common.FileRef{
				Fileid: string(details.Teaser.Int),
				Urls:   urls,
			}
		}

		if details.Banner.Status == pgtype.Present {
			urls, err := GetFileURLs(ctx, neb.Q, int(details.Banner.Int))
			if err != nil {
				return nil, err
			}

			rel.Banner = &common.FileRef{
				Fileid: string(details.Banner.Int),
				Urls:   urls,
			}
		}

		rel.Screenshots = make([]*common.FileRef, len(details.Screenshots.Elements))
		for idx, el := range details.Screenshots.Elements {
			urls, err := GetFileURLs(ctx, neb.Q, int(el.Int))
			if err != nil {
				return nil, err
			}

			rel.Screenshots[idx] = &common.FileRef{
				Fileid: string(el.Int),
				Urls:   urls,
			}
		}

		rel.Videos = make([]string, len(details.Videos.Elements))
		for idx, el := range details.Videos.Elements {
			rel.Videos[idx] = el.String
		}

		result.Releases = append(result.Releases, rel)
	}

	return result, nil
}
