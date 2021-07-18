package storage

import (
	"context"
	"sort"
	"sync"

	"github.com/rotisserie/eris"
	bolt "go.etcd.io/bbolt"
	"google.golang.org/protobuf/proto"

	"github.com/ngld/knossos/packages/api/client"
	"github.com/ngld/knossos/packages/api/common"
)

type ModProvider interface {
	GetVersionsForMod(context.Context, string) ([]string, error)
	GetModRelease(context.Context, string, string) (*common.Release, error)
	GetMods(context.Context, uint32) ([]*common.Release, error)
	GetMod(context.Context, string) (*common.ModMeta, error)
}

type genericModProvider struct {
	bucket       []byte
	versionIndex *StringListIndex
	typeIndex    *StringListIndex
}

var (
	localModsBucket       = []byte("local_mods")
	userModSettingsBucket = []byte("user_mod_settings")
	localVersionIdx       = NewStringListIndex("local_mod_versions", modVersionSorter)
	// TODO maybe add a Uint8ListIndex?
	localTypeIdx = NewStringListIndex("local_mod_types", nil)
	importMutex  = sync.Mutex{}

	// LocalMods implements a ModProvider to access local mods
	LocalMods = genericModProvider{
		bucket:       localModsBucket,
		versionIndex: localVersionIdx,
		typeIndex:    localTypeIdx,
	}
)

func modVersionSorter(_ string, versions []string) error {
	vcoll, err := NewStringVersionCollection(versions)
	if err != nil {
		return err
	}

	sort.Sort(vcoll)
	return nil
}

func ImportMods(ctx context.Context, callback func(context.Context, func(*common.ModMeta) error, func(*common.Release) error) error) error {
	importMutex.Lock()
	defer importMutex.Unlock()

	return db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(localModsBucket)

		// Remove existing entries
		err := bucket.ForEach(func(k, _ []byte) error {
			return bucket.Delete(k)
		})
		if err != nil {
			return err
		}

		// Clear indexes to make sure nothing uses them during the import since they'd be empty during the initial import.
		localVersionIdx.Clear()
		localTypeIdx.Clear()

		localVersionIdx.StartBatch()
		localTypeIdx.StartBatch()

		ctx = CtxWithTx(ctx, tx)

		// Call the actual import function
		err = callback(ctx, func(mod *common.ModMeta) error {
			encoded, err := proto.Marshal(mod)
			if err != nil {
				return err
			}

			err = bucket.Put([]byte(mod.Modid), encoded)
			if err != nil {
				return err
			}

			// Add this mod to our type index
			localTypeIdx.BatchedAdd(mod.Type.String(), mod.Modid)
			return nil
		}, func(rel *common.Release) error {
			encoded, err := proto.Marshal(rel)
			if err != nil {
				return err
			}

			err = bucket.Put([]byte(rel.Modid+"#"+rel.Version), encoded)
			if err != nil {
				return err
			}

			localVersionIdx.BatchedAdd(rel.Modid, rel.Version)
			return nil
		})
		if err != nil {
			return err
		}

		err = localVersionIdx.FinishBatch(tx)
		if err != nil {
			return err
		}

		return localTypeIdx.FinishBatch(tx)
	})
}

func ImportUserSettings(ctx context.Context, callback func(context.Context, func(string, string, *client.UserSettings) error) error) error {
	return db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(userModSettingsBucket)

		// Remove existing entries
		err := bucket.ForEach(func(k, _ []byte) error {
			return bucket.Delete(k)
		})
		if err != nil {
			return err
		}

		ctx = CtxWithTx(ctx, tx)

		return callback(ctx, func(modID, version string, us *client.UserSettings) error {
			encoded, err := proto.Marshal(us)
			if err != nil {
				return err
			}

			return bucket.Put([]byte(modID+"#"+version), encoded)
		})
	})
}

func SaveLocalMod(ctx context.Context, mod *common.ModMeta) error {
	return update(ctx, func(tx *bolt.Tx) error {
		importMutex.Lock()
		defer importMutex.Unlock()

		bucket := tx.Bucket(localModsBucket)
		encoded, err := proto.Marshal(mod)
		if err != nil {
			return eris.Wrap(err, "failed to serialise mod")
		}

		return bucket.Put([]byte(mod.Modid), encoded)
	})
}

func SaveLocalModRelease(ctx context.Context, release *common.Release) error {
	tx := TxFromCtx(ctx)
	if tx == nil {
		return BatchUpdate(ctx, func(ctx context.Context) error {
			return SaveLocalModRelease(ctx, release)
		})
	}

	importMutex.Lock()
	defer importMutex.Unlock()

	bucket := tx.Bucket(localModsBucket)
	versions := localVersionIdx.Lookup(release.Modid)

	isNew := true
	for _, ver := range versions {
		if ver == release.Version {
			isNew = false
			break
		}
	}

	// Update the version index for new mods
	if isNew {
		err := localVersionIdx.Add(tx, release.Modid, release.Version)
		if err != nil {
			return err
		}
	}

	// Finally, we can save the actual mod
	encoded, err := proto.Marshal(release)
	if err != nil {
		return eris.Wrap(err, "failed to encode release")
	}
	return bucket.Put([]byte(release.Modid+"#"+release.Version), encoded)
}

func (p genericModProvider) GetMods(ctx context.Context, taskRef uint32) ([]*common.Release, error) {
	var result []*common.Release

	err := db.View(func(tx *bolt.Tx) error {
		// Retrieve IDs and the latest version for all known local mods
		bucket := tx.Bucket(p.bucket)
		result = make([]*common.Release, 0)

		return p.versionIndex.ForEach(func(modID string, versions []string) error {
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

func (p genericModProvider) GetMod(ctx context.Context, id string) (*common.ModMeta, error) {
	var mod common.ModMeta
	err := view(ctx, func(tx *bolt.Tx) error {
		encoded := tx.Bucket(p.bucket).Get([]byte(id))
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

func (p genericModProvider) GetModRelease(ctx context.Context, id string, version string) (*common.Release, error) {
	mod := new(common.Release)
	err := db.View(func(tx *bolt.Tx) error {
		item := tx.Bucket(p.bucket).Get([]byte(id + "#" + version))
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

func (p genericModProvider) GetVersionsForMod(ctx context.Context, id string) ([]string, error) {
	result := p.versionIndex.Lookup(id)

	if len(result) < 1 {
		return nil, eris.Errorf("No versions found for mod %s", id)
	}

	return result, nil
}

var _ ModProvider = (*genericModProvider)(nil)

func SaveUserSettingsForMod(ctx context.Context, id, version string, settings *client.UserSettings) error {
	return db.Update(func(tx *bolt.Tx) error {
		encoded, err := proto.Marshal(settings)
		if err != nil {
			return err
		}

		bucket := tx.Bucket(userModSettingsBucket)
		return bucket.Put([]byte(id+"#"+version), encoded)
	})
}

func GetUserSettingsForMod(ctx context.Context, id, version string) (*client.UserSettings, error) {
	result := new(client.UserSettings)
	err := db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(userModSettingsBucket)

		encoded := bucket.Get([]byte(id + "#" + version))
		if encoded != nil {
			return proto.Unmarshal(encoded, result)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}
