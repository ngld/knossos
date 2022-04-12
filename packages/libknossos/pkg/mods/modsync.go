package mods

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ngld/knossos/packages/api/common"
	"github.com/ngld/knossos/packages/libknossos/pkg/api"
	"github.com/ngld/knossos/packages/libknossos/pkg/downloader"
	"github.com/ngld/knossos/packages/libknossos/pkg/helpers"
	"github.com/ngld/knossos/packages/libknossos/pkg/storage"
	"github.com/rotisserie/eris"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type modPackInfo struct {
	mod *common.ModIndex_Mod
	idx int
}

var errModNotFound = eris.New("remote mod not found")

func fetchRemoteMessage(ctx context.Context, messageName string, ref protoreflect.ProtoMessage) error {
	resp, err := helpers.CachedGet(ctx, api.SyncEndpoint+"/"+messageName)
	if err != nil {
		return eris.Wrapf(err, "failed to send modsync request to Nebula (%s)", messageName)
	}
	defer resp.Body.Close()

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
		return eris.Wrapf(err, "failed to read %s", messageName)
	}

	err = proto.Unmarshal(encoded, ref)
	if err != nil {
		return eris.Wrapf(err, "failed to parse response to %s", messageName)
	}

	return nil
}

func calcVersionsChecksum(versions []string) ([]byte, error) {
	// We use simple string sorting here instead of proper versioning sorting since it doesn't matter *how* the versions
	// are sorted as long as they appear in the same order on server and client.
	sort.Strings(versions)

	hasher := sha256.New()
	for _, version := range versions {
		_, err := hasher.Write([]byte(version))
		if err != nil {
			return nil, eris.Wrap(err, "failed to hash mod versions")
		}
	}

	return hasher.Sum(nil), nil
}

func UpdateRemoteModIndex(ctx context.Context) error {
	resyncNeeded := false
	for resyncNeeded {
		resyncNeeded = false
		err := storage.ImportRemoteMods(ctx, func(ctx context.Context, params storage.RemoteImportCallbackParams) error {
			curDates, err := storage.GetRemoteModsLastModifiedDates(ctx)
			if err != nil {
				return err
			}

			settings, err := storage.GetSettings(ctx)
			if err != nil {
				return err
			}

			api.Log(ctx, api.LogInfo, "Fetching remote index")
			var index common.ModIndex
			err = fetchRemoteMessage(ctx, "index", &index)
			if err != nil {
				return eris.Wrap(err, "failed to fetch index")
			}

			if len(index.Mods) == 0 {
				api.Log(ctx, api.LogInfo, "No changes since last check")
				api.SetProgress(ctx, 1, "Done")
				return nil
			}

			packs := make([]*downloader.QueueItem, 0)
			tmpFolder := filepath.Join(settings.LibraryPath, "temp")
			modEntries := make(map[string]modPackInfo)

			// If the mod index hasn't changed since the last time we fetched it, index will be the zero value of common.ModIndex
			// this means that index.Mods is an empty list which means we do nothing which is exactly what we want.
			for _, entry := range index.Mods {
				curModDates, found := curDates[entry.Modid]
				if !found || err != nil {
					curModDates = make([]time.Time, len(entry.PacksLastModified)+1)
				}

				if !curModDates[0].Equal(entry.LastModified.AsTime()) {
					key := "mod-" + entry.Modid
					modEntries[key] = modPackInfo{
						mod: entry,
						idx: -1,
					}
					packs = append(packs, &downloader.QueueItem{
						Key:      key,
						Filepath: filepath.Join(tmpFolder, "modsync-"+key),
						Mirrors:  []string{api.SyncEndpoint + "/m." + entry.Modid},
						Checksum: nil,
						Filesize: 0,
					})
				}

				for idx, modified := range entry.PacksLastModified {
					if len(curModDates) <= idx+1 || !curModDates[idx+1].Equal(modified.AsTime()) {
						key := fmt.Sprintf("pack-%s-%03d", entry.Modid, idx)
						modEntries[key] = modPackInfo{
							mod: entry,
							idx: idx,
						}
						packs = append(packs, &downloader.QueueItem{
							Key:      key,
							Filepath: filepath.Join(tmpFolder, "modsync-"+key),
							Mirrors:  []string{fmt.Sprintf("%s/m.%s.%03d", api.SyncEndpoint, entry.Modid, idx)},
							Checksum: nil,
							Filesize: 0,
						})
					}
				}
			}

			queue, err := downloader.NewQueue(ctx, packs)
			if err != nil {
				return eris.Wrap(err, "failed to initialise downloader")
			}

			// We're downloading a bunch of small files; it's fine to download many in parallel even on slow connections.
			queue.MaxParallel = 5
			go queue.Run(ctx)

			done := 0
			for queue.NextResult() {
				dlItem := queue.Result()
				entry := modEntries[dlItem.Key]
				api.SetProgress(ctx, float32(done)/float32(len(packs)), fmt.Sprintf("Processing mod %s (%d of %d)", entry.mod.Modid, done, len(packs)))

				encoded, err := os.ReadFile(dlItem.Filepath)
				if err != nil {
					return eris.Wrapf(err, "failed to open downloaded file %s", dlItem.Filepath)
				}

				if strings.HasPrefix(dlItem.Key, "mod-") {
					var modMeta common.ModMeta
					err = proto.Unmarshal(encoded, &modMeta)
					if err != nil {
						return eris.Wrapf(err, "failed to parse mod meta %s", dlItem.Key)
					}

					err = params.AddMod(&modMeta)
					if err != nil {
						return eris.Wrapf(err, "failed to save mod metadata for %s", entry.mod.Modid)
					}

					if _, ok := curDates[entry.mod.Modid]; !ok {
						curDates[entry.mod.Modid] = make([]time.Time, 1)
					}

					curDates[entry.mod.Modid][0] = entry.mod.LastModified.AsTime()
				} else {
					var releasePack common.ReleasePack
					err = proto.Unmarshal(encoded, &releasePack)
					if err != nil {
						return eris.Wrapf(err, "failed to parse mod release pack %s", dlItem.Key)
					}

					if releasePack.Modid != entry.mod.Modid || releasePack.Packnum != uint32(entry.idx) {
						return eris.Errorf("received wrong pack! expected %d for %s but got %d for %s", entry.idx, entry.mod.Modid, releasePack.Packnum, releasePack.Modid)
					}

					for _, rel := range releasePack.Releases {
						err := params.AddRelease(rel)
						if err != nil {
							return eris.Wrapf(err, "failed to import release %s for %s", rel.Version, entry.mod.Modid)
						}
					}

					modDates := curDates[entry.mod.Modid]
					for len(modDates) <= entry.idx+1 {
						modDates = append(modDates, time.Time{})
					}

					modDates[entry.idx+1] = entry.mod.PacksLastModified[entry.idx].AsTime()
					curDates[entry.mod.Modid] = modDates
				}

				done++
			}

			err = queue.Error()
			if err != nil {
				return eris.Wrap(err, "failed to download modsync files")
			}

			api.Log(ctx, api.LogInfo, "Looking for removed mods")
			releases, err := storage.RemoteMods.GetMods(ctx)
			if err != nil {
				return eris.Wrap(err, "failed to retrieve mod IDs")
			}

			seen := make(map[string]bool)
			for _, entry := range index.Mods {
				seen[entry.Modid] = true

				localVersions, err := storage.RemoteMods.GetVersionsForMod(ctx, entry.Modid)
				if err == nil {
					versionHash, err := calcVersionsChecksum(localVersions)
					if err != nil {
						return err
					}

					versionMismatch := !bytes.Equal(versionHash, entry.VersionChecksum)
					if versionMismatch {
						api.Log(ctx, api.LogInfo, "Version mismatch detected, clearing releases for %s.", entry.Modid)

						err = params.RemoveModReleases(entry.Modid)
						if err != nil {
							return err
						}

						delete(curDates, entry.Modid)
						resyncNeeded = true
					}
				}
			}

			for _, rel := range releases {
				if !seen[rel.Modid] {
					api.Log(ctx, api.LogInfo, "Removing %s", rel.Modid)
					err = params.RemoveMod(rel.Modid)
					if err != nil {
						return eris.Wrapf(err, "failed to remove mod %s", rel.Modid)
					}
				}
			}

			api.SetProgress(ctx, 1, "Finishing")

			err = storage.UpdateRemoteModsLastModifiedDates(ctx, curDates)
			if err == nil {
				api.Log(ctx, api.LogInfo, "Done")
			}

			return err
		})
		if err != nil {
			return err
		}

		if resyncNeeded {
			api.Log(ctx, api.LogInfo, "Found inconsistencies; will sync again.")
		}
	}

	return nil
}

func FetchModChecksums(ctx context.Context, modVersions map[string]string) (map[string]*common.ChecksumPack, error) {
	tempFolder, err := os.MkdirTemp("", "knossos-chk-dl")
	if err != nil {
		return nil, eris.Wrap(err, "failed to create temp folder for checksums")
	}
	defer os.RemoveAll(tempFolder)

	queueItems := make([]*downloader.QueueItem, 0, len(modVersions))
	for modID, version := range modVersions {
		queueItems = append(queueItems, &downloader.QueueItem{
			Key:      modID,
			Filepath: filepath.Join(tempFolder, fmt.Sprintf("c.%s.%s", modID, version)),
			Mirrors:  []string{fmt.Sprintf("%s/c.%s.%s", api.SyncEndpoint, modID, version)},
		})
	}

	queue, err := downloader.NewQueue(ctx, queueItems)
	if err != nil {
		return nil, eris.Wrap(err, "failed to init download queue")
	}

	err = queue.Run(ctx)
	if err != nil {
		return nil, eris.Wrap(err, "failed to download checksums")
	}

	result := make(map[string]*common.ChecksumPack)
	for _, item := range queueItems {
		encoded, err := os.ReadFile(item.Filepath)
		if err != nil {
			return nil, eris.Wrapf(err, "failed to read downloaded checksum file %s", item.Filepath)
		}

		data := new(common.ChecksumPack)
		err = proto.Unmarshal(encoded, data)
		if err != nil {
			return nil, eris.Wrapf(err, "failed to parse checksum file %s", item.Filepath)
		}

		result[item.Key] = data
	}

	return result, nil
}
