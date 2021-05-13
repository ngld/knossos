package mods

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io/ioutil"
	"sort"
	"time"

	"github.com/ngld/knossos/packages/api/common"
	"github.com/ngld/knossos/packages/libknossos/pkg/api"
	"github.com/ngld/knossos/packages/libknossos/pkg/helpers"
	"github.com/ngld/knossos/packages/libknossos/pkg/storage"
	"github.com/rotisserie/eris"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

var errModNotFound = eris.New("remote mod not found")

func fetchRemoteMessage(ctx context.Context, messageName string, ref protoreflect.ProtoMessage) error {
	resp, err := helpers.CachedGet(ctx, api.SyncEndpoint+"/"+messageName)
	if err != nil {
		return err
	}

	if resp.StatusCode == 404 {
		return errModNotFound
	}

	if resp.StatusCode == 304 {
		return nil
	}

	if resp.StatusCode != 200 {
		return eris.Errorf("request to %s failed with status %d", api.SyncEndpoint+"/"+messageName, resp.StatusCode)
	}

	encoded, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		resp.Body.Close()
		return err
	}
	resp.Body.Close()

	return proto.Unmarshal(encoded, ref)
}

func calcVersionsChecksum(ctx context.Context, versions []string) ([]byte, error) {
	// We use simple string sorting here instead of proper versioning sorting since it doesn't matter *how* the versions
	// are sorted as long as they appear in the same order on server and client.
	sort.Strings(versions)

	hasher := sha256.New()
	for _, version := range versions {
		_, err := hasher.Write([]byte(version))
		if err != nil {
			return nil, err
		}
	}

	return hasher.Sum(nil), nil
}

func processRemoteMod(ctx context.Context, params storage.RemoteImportCallbackParams, entry *common.ModIndex_Mod, packDates []time.Time) ([]time.Time, error) {
	var modMeta common.ModMeta
	err := fetchRemoteMessage(ctx, fmt.Sprintf("m.%s", entry.Modid), &modMeta)
	if err != nil {
		if eris.Is(err, errModNotFound) {
			err = params.RemoveMod(entry.Modid)
			if err != nil {
				return nil, eris.Wrapf(err, "failed to remove mod %s", entry.Modid)
			}

			return nil, nil
		}

		return nil, eris.Wrapf(err, "failed to fetch mod meta for %s", entry.Modid)
	}

	if modMeta.Modid == "" {
		// We got a 304 response which means that nothing changed. Skip
		return packDates, nil
	}

	err = params.AddMod(&modMeta)
	if err != nil {
		return nil, err
	}

	localVersions, err := storage.GetVersionsForRemoteMod(ctx, entry.Modid)
	if err != nil {
		return nil, err
	}

	versionHash, err := calcVersionsChecksum(ctx, localVersions)
	if err != nil {
		return nil, err
	}

	versionMismatch := !bytes.Equal(versionHash, entry.VersionChecksum)

	for idx, curPackDate := range entry.PacksLastModified {
		if !versionMismatch && idx < len(packDates) && !curPackDate.AsTime().After(packDates[idx]) {
			continue
		}

		api.Log(ctx, api.LogInfo, "Fetching pack %d for %s", idx, entry.Modid)
		var pack common.ReleasePack
		err = fetchRemoteMessage(ctx, fmt.Sprintf("m.%s.%03d", entry.Modid, idx), &pack)
		if err != nil {
			return nil, eris.Wrapf(err, "failed to fetch pack %d of %s", idx, entry.Modid)
		}

		if pack.Modid != entry.Modid || pack.Packnum != uint32(idx) {
			return nil, eris.Wrapf(err, "received wrong pack! expected %d for %s but got %d for %s", idx, entry.Modid, pack.Packnum, pack.Modid)
		}

		for _, rel := range pack.Releases {
			err = params.AddRelease(rel)
			if err != nil {
				return nil, eris.Wrapf(err, "failed to import release %s for %s", rel.Version, entry.Modid)
			}
		}

		if idx < len(packDates) {
			packDates[idx] = curPackDate.AsTime()
		} else {
			packDates = append(packDates, curPackDate.AsTime())
		}
	}

	return packDates, nil
}

func UpdateRemoteModIndex(ctx context.Context) error {
	return storage.ImportRemoteMods(ctx, func(ctx context.Context, params storage.RemoteImportCallbackParams) error {
		curDates, err := storage.GetRemoteModsLastModifiedDates(ctx)
		if err != nil {
			return err
		}

		api.Log(ctx, api.LogInfo, "Fetching remote index")
		var index common.ModIndex
		err = fetchRemoteMessage(ctx, "index", &index)
		if err != nil {
			return eris.Wrap(err, "failed to fetch index")
		}

		// If the mod index hasn't changed since the last time we fetched it, index will be the zero value of common.ModIndex
		// this means that index.Mods is an empty list which means we do nothing which is exactly what we want.
		for idx, entry := range index.Mods {
			api.SetProgress(ctx, float32(idx)/float32(len(index.Mods)), fmt.Sprintf("Processing mod %s (%d of %d)", entry.Modid, idx, len(index.Mods)))

			curModDates, found := curDates[entry.Modid]
			if found {
				if !curModDates[0].Equal(entry.LastModified.AsTime()) {
					curModDates, err = processRemoteMod(ctx, params, entry, curModDates[1:])

					curModDates = append([]time.Time{entry.LastModified.AsTime()}, curModDates...)
				}
			} else {
				curModDates, err = processRemoteMod(ctx, params, entry, make([]time.Time, 0))

				curModDates = append([]time.Time{entry.LastModified.AsTime()}, curModDates...)
			}

			if err != nil {
				return err
			}

			curDates[entry.Modid] = curModDates
		}

		api.SetProgress(ctx, 1, "Finishing")

		return storage.UpdateRemoteModsLastModifiedDates(ctx, curDates)
	})
}
