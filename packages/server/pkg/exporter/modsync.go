package exporter

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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
			Fileid: fmt.Sprint(*row.Teaser),
			Urls:   urls,
		}
	}

	if row.Banner != nil {
		urls, err := GetFileURLs(ctx, q, int(*row.Banner))
		if err != nil {
			return nil, err
		}

		rel.Banner = &common.FileRef{
			Fileid: fmt.Sprint(*row.Banner),
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
			Fileid: fmt.Sprint(*el),
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

	depMap := make(map[int32][]queries.GetPublicPackageDependencsByReleaseIDRow)
	for _, dep := range pkgDeps {
		depMap[*dep.PackageID] = append(depMap[*dep.PackageID], dep)
	}

	pkgArchives, err := q.GetPublicPackageArchivesByReleaseID(ctx, *row.ID)
	if err != nil {
		return nil, eris.Wrapf(err, "failed to fetch package archives for release %d (%s)", *row.ID, *row.Modid)
	}

	archiveMap := make(map[int32][]queries.GetPublicPackageArchivesByReleaseIDRow)
	for _, archive := range pkgArchives {
		archiveMap[*archive.PackageID] = append(archiveMap[*archive.PackageID], archive)
	}

	pkgExes, err := q.GetPublicPackageExecutablesByReleaseID(ctx, *row.ID)
	if err != nil {
		return nil, eris.Wrapf(err, "failed to fetch package executables for release %d (%s)", *row.ID, *row.Modid)
	}

	exeMap := make(map[int32][]queries.GetPublicPackageExecutablesByReleaseIDRow)
	for _, exe := range pkgExes {
		exeMap[*exe.PackageID] = append(exeMap[*exe.PackageID], exe)
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
			Size:     uint64(*archive.Filesize),
			Mirrors:  mirrors,
			Files:    make([]*common.ChecksumPack_Archive_File, 0, len(files)),
		}

		for fpath, chksum := range files {
			if fpath == "" {
				continue
			}

			if chksum[:2] != "\\x" {
				return nil, eris.Errorf("failed to decode checksum for %s of archive %d of release %d: %s", fpath, *archive.ID, *row.ID, chksum)
			}
			rawsum, err := hex.DecodeString(chksum[2:])
			if err != nil {
				return nil, eris.Wrapf(err, "failed to decode checksum for %s of archive %d of release %d", fpath, *archive.ID, *row.ID)
			}

			ar.Files = append(ar.Files, &common.ChecksumPack_Archive_File{
				Filename: fpath,
				Checksum: rawsum,
			})
		}

		pack.Archives[*archive.Label] = ar
	}

	encoded, err := proto.Marshal(pack)
	if err != nil {
		return nil, eris.Wrapf(err, "failed to serialise checksum pack for release %d", *row.ID)
	}

	err = ioutil.WriteFile(filepath.Join(storagePath, fmt.Sprintf("c.%s.%s", *row.Modid, *row.Version)), encoded, 0o660)
	if err != nil {
		return nil, eris.Wrapf(err, "failed to write checksum pack for release %d", *row.ID)
	}

	return rel, nil
}

func calcVersionsChecksum(_ context.Context, versions []string) ([]byte, error) {
	// We use simple string sorting here instead of proper versioning sorting since it doesn't matter *how* the versions
	// are sorted as long as they appear in the same order on server and client.
	sort.Strings(versions)

	hasher := sha256.New()
	for _, version := range versions {
		_, err := hasher.Write([]byte(version))
		if err != nil {
			return nil, eris.Wrap(err, "failed to calculate hash")
		}
	}

	return hasher.Sum(nil), nil
}

func writePack(_ context.Context, modID string, storagePath string, packnum uint32, relID int, pack []*common.Release) error {
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
	err = ioutil.WriteFile(packPath, encoded, 0o660)
	if err != nil {
		return eris.Wrapf(err, "failed to write pack %d for release %d", packnum, relID)
	}

	return nil
}

func buildModIndex(ctx context.Context, q queries.Querier, modID string, aID int32, storagePath string) (*common.ModIndex_Mod, error) {
	entry := new(common.ModIndex_Mod)
	entry.Modid = modID
	entry.LastModified = timestamppb.Now()
	entry.PacksLastModified = make([]*timestamppb.Timestamp, 0)

	releases, err := q.GetPublicModReleasesByAid(ctx, aID)
	if err != nil {
		return nil, eris.Wrapf(err, "failed to fetch releases for %s", modID)
	}

	versionNumbers := make([]string, len(releases))
	pack := make([]*common.Release, 0, packSize)
	current := uint32(0)
	for idx, release := range releases {
		versionNumbers[idx] = *release.Version

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
			entry.PacksLastModified = append(entry.PacksLastModified, timestamppb.Now())
			current++
		}
	}

	if len(pack) > 0 {
		err = writePack(ctx, modID, storagePath, current, -1, pack)
		if err != nil {
			return nil, err
		}
		entry.PacksLastModified = append(entry.PacksLastModified, timestamppb.Now())
	}

	vchk, err := calcVersionsChecksum(ctx, versionNumbers)
	if err != nil {
		return nil, err
	}
	entry.VersionChecksum = vchk
	return entry, nil
}

func updateModIndex(ctx context.Context, q queries.Querier, entry *common.ModIndex_Mod, aID int32, storagePath string) error {
	var lastUpdate time.Time

	for _, modifiedRaw := range entry.PacksLastModified {
		modified := modifiedRaw.AsTime()
		if modified.After(lastUpdate) {
			lastUpdate = modified
		}
	}

	versionRows, err := q.GetPublicModVersionsByAID(ctx, aID)
	if err != nil {
		return eris.Wrapf(err, "failed to retrieve public release versions for mod %s", entry.Modid)
	}

	versions := make(map[string]bool)
	for _, row := range versionRows {
		versions[*row] = true
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
	versionNumbers := make([]string, 0, 10*len(entry.PacksLastModified))
	var pack common.ReleasePack
	for packnum := range entry.PacksLastModified {
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

				versionNumbers = append(versionNumbers, pbRel.Version)
			} else {
				deleted := !versions[version]
				if !deleted {
					delIdx := sort.SearchStrings(deletedRels, version)
					deleted = delIdx < len(deletedRels) && deletedRels[delIdx] == version
				}

				if deleted {
					pack.Releases = append(pack.Releases[:idx], pack.Releases[idx+1:]...)
					modified = true
				} else {
					versionNumbers = append(versionNumbers, version)
				}
			}
		}

		if modified {
			entry.PacksLastModified[packnum] = timestamppb.Now()

			encoded, err = proto.Marshal(&pack)
			if err != nil {
				return eris.Wrapf(err, "failed to serialise pack %d from mod %s", packnum, entry.Modid)
			}

			err = ioutil.WriteFile(packPath, encoded, 0o660)
			if err != nil {
				return eris.Wrapf(err, "failed to write pack %d from mod %s", packnum, entry.Modid)
			}
		}
	}

	// add new entries
	current := uint32(len(entry.PacksLastModified) - 1)
	encoded, err := ioutil.ReadFile(fmt.Sprintf("m.%s.%03d", entry.Modid, current))
	if err != nil && !eris.Is(err, os.ErrNotExist) {
		return eris.Wrapf(err, "failed to open last pack (%d) from mod %s", current, entry.Modid)
	}

	var rels []*common.Release
	if !eris.Is(err, os.ErrNotExist) {
		err = proto.Unmarshal(encoded, &pack)
		if err != nil {
			return eris.Wrapf(err, "failed to deserialise last pack (%d) from mod %s", current, entry.Modid)
		}

		rels = pack.Releases
		for _, pbRel := range convertedRels {
			versionNumbers = append(versionNumbers, pbRel.Version)
			rels = append(rels, pbRel)
			if len(rels) >= packSize {
				err = writePack(ctx, entry.Modid, storagePath, current, -1, rels)
				if err != nil {
					return err
				}

				rels = make([]*common.Release, 0, packSize)
				if len(entry.PacksLastModified) <= int(current) {
					entry.PacksLastModified = append(entry.PacksLastModified, timestamppb.Now())
				} else {
					entry.PacksLastModified[current] = timestamppb.Now()
				}
				current++
			}
		}
	}

	if len(rels) > 0 {
		err = writePack(ctx, entry.Modid, storagePath, current, -1, rels)
		if err != nil {
			return err
		}

		if len(entry.PacksLastModified) <= int(current) {
			entry.PacksLastModified = append(entry.PacksLastModified, timestamppb.Now())
		} else {
			entry.PacksLastModified[current] = timestamppb.Now()
		}
	}

	vchk, err := calcVersionsChecksum(ctx, versionNumbers)
	if err != nil {
		return err
	}

	entry.VersionChecksum = vchk
	return err
}

func writeModMetaFile(_ context.Context, mod queries.GetPublicModUpdatedDatesRow, storagePath string) error {
	// We always update the mod metadata files since they're small and fast to write and we don't have a marker
	// to check for updates.
	modMeta := &common.ModMeta{
		Modid: *mod.Modid,
		Title: *mod.Title,
		Tags:  mod.Tags,
	}

	switch db.ModType(*mod.Type) {
	case db.TypeMod:
		modMeta.Type = common.ModType_MOD
	case db.TypeTotalConversion:
		modMeta.Type = common.ModType_TOTAL_CONVERSION
	case db.TypeEngine:
		modMeta.Type = common.ModType_ENGINE
	case db.TypeTool:
		modMeta.Type = common.ModType_TOOL
	case db.TypeExtension:
		modMeta.Type = common.ModType_EXTENSION
	default:
		return eris.Errorf("failed to parse mod type for mod %s", *mod.Modid)
	}

	encoded, err := proto.Marshal(modMeta)
	if err != nil {
		return eris.Wrapf(err, "failed to serialise metadata for mod %s", *mod.Modid)
	}

	err = ioutil.WriteFile(filepath.Join(storagePath, fmt.Sprintf("m.%s", *mod.Modid)), encoded, 0o660)
	if err != nil {
		return eris.Wrapf(err, "failed to write mod metadata file for mod %s", *mod.Modid)
	}

	return nil
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
		idxEntry, found := modMap[*mod.Modid]
		if !found {
			// The mod isn't listed in the index. Build the pack files and append it.
			entry, err := buildModIndex(ctx, q, *mod.Modid, *mod.Aid, storagePath)
			if err != nil {
				return eris.Wrapf(err, "failed to build index for %s", *mod.Modid)
			}

			index.Mods = append(index.Mods, entry)
			err = writeModMetaFile(ctx, mod, storagePath)
			if err != nil {
				return eris.Wrapf(err, "failed to write mod meta for %s", *mod.Modid)
			}

			continue
		}

		var lastIdxUpdate time.Time
		for _, stamp := range idxEntry.PacksLastModified {
			stampTime := stamp.AsTime()
			if stampTime.After(lastIdxUpdate) {
				lastIdxUpdate = stampTime
			}
		}

		if mod.Updated.Time.After(idxEntry.LastModified.AsTime()) {
			err = writeModMetaFile(ctx, mod, storagePath)
			if err != nil {
				return err
			}

			idxEntry.LastModified = timestamppb.Now()
		}

		// Check if the mod metadata has changed since the last time we updated packs for this mod.
		if mod.ReleaseUpdated.Time.After(lastIdxUpdate) {
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

	err = ioutil.WriteFile(indexFile, encoded, 0o660)
	if err != nil {
		return eris.Wrapf(err, "failed to write mod index to %s", indexFile)
	}

	return nil
}
