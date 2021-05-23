package storage

import (
	"context"
	"encoding/json"
	"time"

	"github.com/rotisserie/eris"
	bolt "go.etcd.io/bbolt"
	"google.golang.org/protobuf/proto"

	"github.com/ngld/knossos/packages/api/common"
)

var (
	remoteModsBucket = []byte("remote_mods")
	remoteVersionIdx = NewStringListIndex("remote_mod_versions", modVersionSorter)
	// TODO maybe add a Uint8ListIndex?
	remoteTypeIdx = NewStringListIndex("remote_mod_types", nil)
)

type RemoteIndexLastModifiedDates map[string][]time.Time

type RemoteImportCallbackParams struct {
	ForAllVersions func(func(string, []string) error) error
	RemoveMod      func(string) error
	RemoveRelease  func(string, string) error
	AddMod         func(*common.ModMeta) error
	AddRelease     func(*common.Release) error
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
						return err
					}
				}

				remoteVersionIdx.BatchedRemoveAll(id)
				return bucket.Delete([]byte(id))
			},
			RemoveRelease: func(id, version string) error {
				err := bucket.Delete([]byte(id + "#" + version))
				if err != nil {
					return err
				}

				remoteVersionIdx.BatchedRemove(id, version)
				return nil
			},
			AddMod: func(mod *common.ModMeta) error {
				encoded, err := proto.Marshal(mod)
				if err != nil {
					return err
				}

				err = bucket.Put([]byte(mod.Modid), encoded)
				if err != nil {
					return err
				}

				// Add this mod to our type index
				remoteTypeIdx.BatchedAdd(mod.Type.String(), mod.Modid)
				return nil
			},
			AddRelease: func(rel *common.Release) error {
				encoded, err := proto.Marshal(rel)
				if err != nil {
					return err
				}

				err = bucket.Put([]byte(rel.Modid+"#"+rel.Version), encoded)
				if err != nil {
					return err
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

		return json.Unmarshal(encoded, &result)
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
			return err
		}

		return tx.Bucket(remoteModsBucket).Put([]byte("#last_modifieds"), encoded)
	})
}

func GetRemoteMods(ctx context.Context, taskRef uint32) ([]*common.Release, error) {
	var result []*common.Release

	err := db.View(func(tx *bolt.Tx) error {
		// Retrieve IDs and the latest version for all known remote mods
		bucket := tx.Bucket(remoteModsBucket)
		result = make([]*common.Release, 0)

		return remoteVersionIdx.ForEach(func(modID string, versions []string) error {
			item := bucket.Get([]byte(modID + "#" + versions[len(versions)-1]))
			if item == nil {
				return eris.Errorf("Failed to find mod %s from index", modID+"#"+versions[len(versions)-1])
			}

			meta := new(common.Release)
			err := proto.Unmarshal(item, meta)
			if err != nil {
				return err
			}

			result = append(result, meta)
			return nil
		})
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func GetRemoteMod(ctx context.Context, id string) (*common.ModMeta, error) {
	var mod common.ModMeta
	err := view(ctx, func(tx *bolt.Tx) error {
		encoded := tx.Bucket(remoteModsBucket).Get([]byte(id))
		if encoded == nil {
			return eris.New("mod not found")
		}

		return proto.Unmarshal(encoded, &mod)
	})
	if err != nil {
		return nil, err
	}
	return &mod, nil
}

func GetRemoteModRelease(ctx context.Context, id string, version string) (*common.Release, error) {
	mod := new(common.Release)
	err := db.View(func(tx *bolt.Tx) error {
		item := tx.Bucket(remoteModsBucket).Get([]byte(id + "#" + version))
		if item == nil {
			return eris.New("mod not found")
		}

		return proto.Unmarshal(item, mod)
	})
	if err != nil {
		return nil, err
	}
	return mod, nil
}

func GetVersionsForRemoteMod(ctx context.Context, id string) ([]string, error) {
	result := remoteVersionIdx.Lookup(id)

	if len(result) < 1 {
		return nil, eris.Errorf("No versions found for mod %s", id)
	}

	return result, nil
}

type RemoteMods struct{}

var _ ModProvider = (*RemoteMods)(nil)

func (RemoteMods) GetVersionsForMod(id string) ([]string, error) {
	return GetVersionsForRemoteMod(context.Background(), id)
}

func (RemoteMods) GetModMetadata(id, version string) (*common.Release, error) {
	return GetRemoteModRelease(context.Background(), id, version)
}
