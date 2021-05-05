package exporter

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/jackc/pgtype"
	"github.com/ngld/knossos/packages/api/common"
	"github.com/ngld/knossos/packages/server/pkg/db"
	"github.com/ngld/knossos/packages/server/pkg/db/queries"
	"github.com/rotisserie/eris"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const packSize = 10

func buildReleaseFromRow(ctx context.Context, q queries.Querier, row queries.GetPublicModReleasesByAidRow, storagePath string) (*common.Release, error) {
	rel := &common.Release{
		Modid:         *row.Modid,
		Version:       *row.Version,
		Folder:        *row.Modid + "-" + *row.Version,
		Description:   *row.Description,
		ReleaseThread: *row.ReleaseThread,
		Released:      timestamppb.New(row.Released.Time),
		Updated:       timestamppb.New(row.Updated.Time),
		Notes:         *row.Notes,
		Videos:        row.Videos,
		Cmdline:       *row.Cmdline,
		ModOrder:      row.ModOrder,
	}

	switch db.EngineStability(*row.Stability) {
	case db.EngineStable:
		rel.Stability = common.ReleaseStability_STABLE
	case db.EngineRC:
		rel.Stability = common.ReleaseStability_RC
	case db.EngineNightly:
		rel.Stability = common.ReleaseStability_NIGHTLY
	case db.EngineUnknown:
	}

	if row.Teaser != nil {
		urls, err := GetFileURLs(ctx, q, int(*row.Teaser))
		if err != nil {
			return nil, err
		}

		rel.Teaser = &common.FileRef{
			Fileid: string(*row.Teaser),
			Urls:   urls,
		}
	}

	if row.Banner != nil {
		urls, err := GetFileURLs(ctx, q, int(*row.Banner))
		if err != nil {
			return nil, err
		}

		rel.Banner = &common.FileRef{
			Fileid: string(*row.Banner),
			Urls:   urls,
		}
	}

	rel.Screenshots = make([]*common.FileRef, len(row.Screenshots))
	for idx, el := range row.Screenshots {
		urls, err := GetFileURLs(ctx, q, int(*el))
		if err != nil {
			return nil, err
		}

		rel.Screenshots[idx] = &common.FileRef{
			Fileid: string(*el),
			Urls:   urls,
		}
	}

	pkgs, err := q.GetPublicPackagesByReleaseID(ctx, *row.ID)
	if err != nil {
		return nil, eris.Wrapf(err, "failed to fetch packages for release %d (%s)", *row.ID, *row.Modid)
	}

	pkgDeps, err := q.GetPublicPackageDependencsByReleaseID(ctx, *row.ID)
	if err != nil {
		return nil, eris.Wrapf(err, "failed to fetch package dependencies for %d (%s)", *row.ID, *row.Modid)
	}

	depMap := make(map[int32][]*queries.GetPublicPackageDependencsByReleaseIDRow)
	for _, dep := range pkgDeps {
		depMap[*dep.PackageID] = append(depMap[*dep.PackageID], &dep)
	}

	pkgArchives, err := q.GetPublicPackageArchivesByReleaseID(ctx, *row.ID)
	if err != nil {
		return nil, eris.Wrapf(err, "failed to fetch package archives for release %d (%s)", *row.ID, *row.Modid)
	}

	archiveMap := make(map[int32][]*queries.GetPublicPackageArchivesByReleaseIDRow)
	for _, archive := range pkgArchives {
		archiveMap[*archive.PackageID] = append(archiveMap[*archive.PackageID], &archive)
	}

	pkgExes, err := q.GetPublicPackageExecutablesByReleaseID(ctx, *row.ID)
	if err != nil {
		return nil, eris.Wrapf(err, "failed to fetch package executables for release %d (%s)", *row.ID, *row.Modid)
	}

	exeMap := make(map[int32][]*queries.GetPublicPackageExecutablesByReleaseIDRow)
	for _, exe := range pkgExes {
		exeMap[*exe.PackageID] = append(exeMap[*exe.PackageID], &exe)
	}

	rel.Packages = make([]*common.Package, len(pkgs))
	for idx, pkg := range pkgs {
		rel.Packages[idx] = &common.Package{
			Name:      *pkg.Name,
			Folder:    *pkg.Folder,
			Notes:     *pkg.Notes,
			KnossosVp: *pkg.KnossosVp,
		}

		relPkg := rel.Packages[idx]

		switch db.PackageType(*pkg.Type) {
		case db.PackageOptional:
			relPkg.Type = common.PackageType_OPTIONAL
		case db.PackageRecommended:
			relPkg.Type = common.PackageType_RECOMMENDED
		case db.PackageRequired:
			relPkg.Type = common.PackageType_REQUIRED
		}

		relPkg.CpuSpec = &common.CpuSpec{
			RequiredFeatures: pkg.CpuSpecs,
		}

		deps := depMap[*pkg.ID]
		relPkg.Dependencies = make([]*common.Dependency, len(deps))
		for idx, dep := range deps {
			relPkg.Dependencies[idx] = &common.Dependency{
				Modid:      *dep.Modid,
				Constraint: *dep.Version,
				Packages:   dep.Packages,
			}
		}

		exes := exeMap[*pkg.ID]
		relPkg.Executables = make([]*common.EngineExecutable, len(exes))
		for idx, exe := range exes {
			relPkg.Executables[idx] = &common.EngineExecutable{
				Path:     *exe.Path,
				Label:    *exe.Label,
				Priority: uint32(*exe.Priority),
				Debug:    *exe.Debug,
			}
		}

		archives := archiveMap[*pkg.ID]
		relPkg.Archives = make([]*common.PackageArchive, len(archives))
		for idx, archive := range archives {
			relPkg.Archives[idx] = &common.PackageArchive{
				Id:          string(*archive.ID),
				Label:       *archive.Label,
				Destination: *archive.Destination,
				Checksum: &common.Checksum{
					Algo:   *archive.ChecksumAlgo,
					Digest: archive.ChecksumDigest.Bytes,
				},
				Filesize: uint64(*archive.Filesize),
			}
		}
	}

	checksums, err := q.GetChecksumsByReleaseID(ctx, *row.ID)
	if err != nil {
		return nil, eris.Wrapf(err, "failed to fetch checksums for release %d", *row.ID)
	}

	pack := &common.ChecksumPack{
		Archives: make(map[string]*common.ChecksumPack_Archive),
	}
	for _, archive := range checksums {
		mirrors, err := GetFileURLsFromValues(ctx, int(*archive.Fid), *archive.StorageKey, archive.External)
		if err != nil {
			return nil, eris.Wrapf(err, "failed to fetch mirrors for archive %d of release %d", *archive.ID, *row.ID)
		}

		files := make(map[string]string)
		err = archive.Files.AssignTo(&files)
		if err != nil {
			return nil, eris.Wrapf(err, "failed to decode filelist for archive %d of release %d", *archive.ID, *row.ID)
		}

		ar := &common.ChecksumPack_Archive{
			Checksum: archive.ChecksumDigest.Bytes,
			Size:     uint32(*archive.Filesize),
			Mirrors:  mirrors,
			Files:    make([]*common.ChecksumPack_Archive_File, len(files)),
		}

		for fpath, chksum := range files {
			ar.Files = append(ar.Files, &common.ChecksumPack_Archive_File{
				Filename: fpath,
				Checksum: []byte(chksum),
			})
		}

		pack.Archives[*archive.Label] = ar
	}

	encoded, err := proto.Marshal(pack)
	if err != nil {
		return nil, eris.Wrapf(err, "failed to serialise checksum pack for release %d", *row.ID)
	}

	err = ioutil.WriteFile(filepath.Join(storagePath, fmt.Sprintf("c.%s.%s", *row.Modid, *row.Version)), encoded, 0660)
	if err != nil {
		return nil, eris.Wrapf(err, "failed to write checksum pack for release %d", *row.ID)
	}

	return rel, nil
}

func writePack(ctx context.Context, modID string, storagePath string, packnum uint32, relID int, pack []*common.Release) error {
	modpack := &common.ReleasePack{
		Modid:    modID,
		Packnum:  packnum,
		Releases: pack,
	}
	encoded, err := proto.Marshal(modpack)
	if err != nil {
		return eris.Wrapf(err, "failed to serialize pack %d of release %d", packnum, relID)
	}

	packPath := filepath.Join(storagePath, fmt.Sprintf("m.%s.%03d", modID, packnum))
	err = ioutil.WriteFile(packPath, encoded, 0660)
	if err != nil {
		return eris.Wrapf(err, "failed to write pack %d for release %d", packnum, relID)
	}

	return nil
}

func buildModIndex(ctx context.Context, q queries.Querier, modID string, aID int32, storagePath string) (*common.ModIndex_Mod, error) {
	entry := new(common.ModIndex_Mod)
	entry.Modid = modID
	entry.LastModified = make([]*timestamppb.Timestamp, 0)

	releases, err := q.GetPublicModReleasesByAid(ctx, aID)
	if err != nil {
		return nil, eris.Wrapf(err, "failed to fetch releases for %s", modID)
	}

	pack := make([]*common.Release, 0, packSize)
	current := uint32(0)
	for _, release := range releases {
		pbRel, err := buildReleaseFromRow(ctx, q, release, storagePath)
		if err != nil {
			return nil, eris.Wrapf(err, "failed to process release %d", *release.ID)
		}

		pack = append(pack, pbRel)
		if len(pack) >= packSize {
			err = writePack(ctx, modID, storagePath, current, int(*release.ID), pack)
			if err != nil {
				return nil, err
			}

			pack = make([]*common.Release, 0, packSize)
			entry.LastModified = append(entry.LastModified, timestamppb.Now())
			current++
		}
	}

	if len(pack) > 0 {
		err = writePack(ctx, modID, storagePath, current, -1, pack)
		if err != nil {
			return nil, err
		}
		entry.LastModified = append(entry.LastModified, timestamppb.Now())
	}

	return entry, nil
}

func updateModIndex(ctx context.Context, q queries.Querier, entry *common.ModIndex_Mod, aID int32, storagePath string) error {
	var lastUpdate time.Time

	for _, modifiedRaw := range entry.LastModified {
		modified := modifiedRaw.AsTime()
		if modified.After(lastUpdate) {
			lastUpdate = modified
		}
	}

	releases, err := q.GetPublicModReleasesByAIDSince(ctx, aID, pgtype.Timestamptz{
		Time:   lastUpdate,
		Status: pgtype.Present,
	})
	if err != nil {
		return eris.Wrapf(err, "failed to retrieve updated/new releases for mod %s", entry.Modid)
	}

	convertedRels := make(map[string]*common.Release)
	deletedRels := make([]string, 0)
	for _, release := range releases {
		if *release.Deleted {
			deletedRels = append(deletedRels, *release.Version)
		} else {
			pbRel, err := buildReleaseFromRow(ctx, q, queries.GetPublicModReleasesByAidRow(release), storagePath)
			if err != nil {
				return err
			}

			convertedRels[*release.Version] = pbRel
		}
	}

	sort.Strings(deletedRels)

	// update / remove existing entries
	var pack common.ReleasePack
	for packnum := range entry.LastModified {
		packPath := filepath.Join(storagePath, fmt.Sprintf("m.%s.%03d", entry.Modid, packnum))
		encoded, err := ioutil.ReadFile(packPath)
		if err != nil {
			return eris.Wrapf(err, "failed to read pack %d from mod %s", packnum, entry.Modid)
		}

		err = proto.Unmarshal(encoded, &pack)
		if err != nil {
			return eris.Wrapf(err, "failed to deserialise pack %d from mod %s", packnum, entry.Modid)
		}

		modified := false
		for idx := len(pack.Releases) - 1; idx >= 0; idx-- {
			version := pack.Releases[idx].Version
			pbRel, found := convertedRels[version]
			if found {
				pack.Releases[idx] = pbRel
				delete(convertedRels, version)
				modified = true
			} else {
				delIdx := sort.SearchStrings(deletedRels, version)
				if delIdx < len(deletedRels) && deletedRels[delIdx] == version {
					pack.Releases = append(pack.Releases[:idx], pack.Releases[idx+1:]...)
					modified = true
				}
			}
		}

		if modified {
			entry.LastModified[packnum] = timestamppb.Now()

			encoded, err = proto.Marshal(&pack)
			if err != nil {
				return eris.Wrapf(err, "failed to serialise pack %d from mod %s", packnum, entry.Modid)
			}

			err = ioutil.WriteFile(packPath, encoded, 0660)
			if err != nil {
				return eris.Wrapf(err, "failed to write pack %d from mod %s", packnum, entry.Modid)
			}
		}
	}

	// add new entries
	current := uint32(len(entry.LastModified) - 1)
	encoded, err := ioutil.ReadFile(fmt.Sprintf("m.%s.%03d", entry.Modid, current))
	if err != nil {
		return eris.Wrapf(err, "failed to open last pack (%d) from mod %s", current, entry.Modid)
	}

	err = proto.Unmarshal(encoded, &pack)
	if err != nil {
		return eris.Wrapf(err, "failed to deserialise last pack (%d) from mod %s", current, entry.Modid)
	}

	rels := pack.Releases
	for _, pbRel := range convertedRels {
		rels = append(rels, pbRel)
		if len(rels) >= packSize {
			err = writePack(ctx, entry.Modid, storagePath, current, -1, rels)
			if err != nil {
				return err
			}

			rels = make([]*common.Release, 0, packSize)
			if len(entry.LastModified) <= int(current) {
				entry.LastModified = append(entry.LastModified, timestamppb.Now())
			} else {
				entry.LastModified[current] = timestamppb.Now()
			}
			current++
		}
	}

	if len(rels) > 0 {
		err = writePack(ctx, entry.Modid, storagePath, current, -1, rels)
		if len(entry.LastModified) <= int(current) {
			entry.LastModified = append(entry.LastModified, timestamppb.Now())
		} else {
			entry.LastModified[current] = timestamppb.Now()
		}
	}

	return err
}

func UpdateModsyncExport(ctx context.Context, q queries.Querier, storagePath string) error {
	indexFile := filepath.Join(storagePath, "index")
	data, err := os.ReadFile(indexFile)
	if err != nil && !eris.Is(err, os.ErrNotExist) {
		return eris.Wrapf(err, "failed to read index from %s", indexFile)
	}

	var index common.ModIndex
	if data != nil {
		err = proto.Unmarshal(data, &index)
		if err != nil {
			return eris.Wrapf(err, "failed to parse %s", indexFile)
		}
	}

	modlist, err := q.GetPublicModUpdatedDates(ctx)
	if err != nil {
		return eris.Wrap(err, "failed to fetch public mods from DB")
	}

	modMap := make(map[string]*common.ModIndex_Mod)
	for _, mod := range index.GetMods() {
		modMap[mod.Modid] = mod
	}

	dbModIDs := make([]string, len(modlist))
	for idx, mod := range modlist {
		dbModIDs[idx] = *mod.Modid

		// We always update the mod metadata files since they're small and fast to write and we don't have a marker
		// to check for updates.
		modMeta := &common.ModMeta{
			Modid: *mod.Modid,
			Title: *mod.Title,
			Tags:  mod.Tags,
		}
		encoded, err := proto.Marshal(modMeta)
		if err != nil {
			return eris.Wrapf(err, "failed to serialise metadata for mod %s", *mod.Modid)
		}

		err = ioutil.WriteFile(filepath.Join(storagePath, fmt.Sprintf("m.%s", *mod.Modid)), encoded, 0660)
		if err != nil {
			return eris.Wrapf(err, "failed to write mod metadata file for mod %s", *mod.Modid)
		}

		idxEntry, found := modMap[*mod.Modid]
		if !found {
			// The mod isn't listed in the index. Build the pack files and append it.
			entry, err := buildModIndex(ctx, q, *mod.Modid, *mod.Aid, storagePath)
			if err != nil {
				return eris.Wrapf(err, "failed to build index for %s", *mod.Modid)
			}

			index.Mods = append(index.Mods, entry)
			continue
		}

		// Check if the mod metadata has changed since the last time we updated packs for this mod.
		isCurrent := false
		for _, idxModified := range idxEntry.LastModified {
			if idxModified != nil && !mod.Updated.Time.After(idxModified.AsTime()) {
				isCurrent = true
				break
			}
		}

		if !isCurrent {
			err = updateModIndex(ctx, q, idxEntry, *mod.Aid, storagePath)
			if err != nil {
				return err
			}
		}
	}

	sort.Strings(dbModIDs)

	for idx := len(index.Mods) - 1; idx >= 0; idx-- {
		IDidx := sort.SearchStrings(dbModIDs, index.Mods[idx].Modid)
		if IDidx >= len(dbModIDs) || dbModIDs[IDidx] != index.Mods[idx].Modid {
			// mod is in index but not in DB; remove
			index.Mods = append(index.Mods[:idx], index.Mods[idx+1:]...)
		}
	}

	encoded, err := proto.Marshal(&index)
	if err != nil {
		return eris.Wrap(err, "failed to serialize mod index")
	}

	err = ioutil.WriteFile(indexFile, encoded, 0660)
	if err != nil {
		return eris.Wrapf(err, "failed to write mod index to %s", indexFile)
	}

	return nil
}
