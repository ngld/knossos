package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"time"

	bolt "go.etcd.io/bbolt"
	"google.golang.org/protobuf/proto"

	"github.com/ngld/knossos/packages/api/common"
	"github.com/rotisserie/eris"
)

var (
	remoteModsBucket = []byte("remote_mods")
	remoteVersionIdx = NewStringListIndex("remote_mod_versions", modVersionSorter)
	// TODO maybe add a Uint8ListIndex?
	remoteTypeIdx = NewStringListIndex("remote_mod_types", nil)

	// RemoteMods implements a ModProvider to access remote mods
	RemoteMods = genericModProvider{
		bucket:       remoteModsBucket,
		versionIndex: remoteVersionIdx,
		typeIndex:    remoteTypeIdx,
	}
)

type RemoteIndexLastModifiedDates map[string][]time.Time

type RemoteImportCallbackParams struct {
	ForAllVersions    func(func(string, []string) error) error
	RemoveMod         func(string) error
	RemoveRelease     func(string, string) error
	RemoveModReleases func(string) error
	AddMod            func(*common.ModMeta) error
	AddRelease        func(*common.Release) error
}

func ImportRemoteMods(ctx context.Context, callback func(context.Context, RemoteImportCallbackParams) error) error {
	return db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(remoteModsBucket)

		remoteVersionIdx.StartBatch()
		remoteTypeIdx.StartBatch()

		ctx = CtxWithTx(ctx, tx)

		// Call the actual import function
		err := callback(ctx, RemoteImportCallbackParams{
			ForAllVersions: remoteVersionIdx.ForEach,
			RemoveMod: func(id string) error {
				for _, version := range remoteVersionIdx.Lookup(id) {
					err := bucket.Delete([]byte(id + "#" + version))
					if err != nil {
						return eris.Wrapf(err, "failed to delete mod release entry %s %s", id, version)
					}
				}

				remoteVersionIdx.BatchedRemoveAll(id)
				err := bucket.Delete([]byte(id))
				if err != nil {
					return eris.Wrapf(err, "failed to delete mod entry %s", id)
				}

				return nil
			},
			RemoveRelease: func(id, version string) error {
				err := bucket.Delete([]byte(id + "#" + version))
				if err != nil {
					return eris.Wrapf(err, "failed to delete mod release %s %s", id, version)
				}

				remoteVersionIdx.BatchedRemove(id, version)
				return nil
			},
			RemoveModReleases: func(id string) error {
				prefix := []byte(id + "#")
				return bucket.ForEach(func(k, v []byte) error {
					if bytes.HasPrefix(k, prefix) {
						err := bucket.Delete(k)
						if err != nil {
							return eris.Wrapf(err, "failed to delete mod release %s", k)
						}
					}

					return nil
				})
			},
			AddMod: func(mod *common.ModMeta) error {
				encoded, err := proto.Marshal(mod)
				if err != nil {
					return eris.Wrapf(err, "failed to serialise mod %s", mod.Modid)
				}

				err = bucket.Put([]byte(mod.Modid), encoded)
				if err != nil {
					return eris.Wrapf(err, "failed to save mod %s", mod.Modid)
				}

				// Add this mod to our type index
				remoteTypeIdx.BatchedAdd(mod.Type.String(), mod.Modid)
				return nil
			},
			AddRelease: func(rel *common.Release) error {
				encoded, err := proto.Marshal(rel)
				if err != nil {
					return eris.Wrapf(err, "failed to serialise mod release %s %s", rel.Modid, rel.Version)
				}

				err = bucket.Put([]byte(rel.Modid+"#"+rel.Version), encoded)
				if err != nil {
					return eris.Wrapf(err, "failed to save mod release %s %s", rel.Modid, rel.Version)
				}

				remoteVersionIdx.BatchedAdd(rel.Modid, rel.Version)
				return nil
			},
		})
		if err != nil {
			return err
		}

		err = remoteVersionIdx.ForEach(func(ID string, versions []string) error {
			count := len(versions)
			for _, version := range versions {
				rel := bucket.Get([]byte(ID + "#" + version))
				if rel == nil {
					remoteVersionIdx.BatchedRemove(ID, version)
					count--
				}
			}

			if count == 0 {
				remoteVersionIdx.BatchedRemoveAll(ID)
			}
			return nil
		})
		if err != nil {
			return err
		}

		err = remoteTypeIdx.ForEach(func(modType string, IDs []string) error {
			for _, ID := range IDs {
				info := bucket.Get([]byte(ID))
				if info == nil {
					remoteTypeIdx.BatchedRemove(modType, ID)
				}
			}

			return nil
		})
		if err != nil {
			return err
		}

		err = remoteVersionIdx.FinishBatch(tx)
		if err != nil {
			return err
		}

		return remoteTypeIdx.FinishBatch(tx)
	})
}

func GetRemoteModsLastModifiedDates(ctx context.Context) (RemoteIndexLastModifiedDates, error) {
	result := make(RemoteIndexLastModifiedDates)
	err := view(ctx, func(tx *bolt.Tx) error {
		encoded := tx.Bucket(remoteModsBucket).Get([]byte("#last_modifieds"))
		if encoded == nil {
			return nil
		}

		err := json.Unmarshal(encoded, &result)
		if err != nil {
			return eris.Wrap(err, "failed to deserialise last modified dates for remote mods")
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func UpdateRemoteModsLastModifiedDates(ctx context.Context, dates RemoteIndexLastModifiedDates) error {
	return update(ctx, func(tx *bolt.Tx) error {
		encoded, err := json.Marshal(&dates)
		if err != nil {
			return eris.Wrap(err, "failed to serialise last modified dates for remote mods")
		}

		err = tx.Bucket(remoteModsBucket).Put([]byte("#last_modifieds"), encoded)
		if err != nil {
			return eris.Wrap(err, "failed to save last modified dates for remote mods")
		}

		return nil
	})
}
