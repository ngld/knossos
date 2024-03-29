package storage

import (
	"bytes"
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
	GetMods(context.Context) ([]*common.Release, error)
	GetAllReleases(context.Context) ([]*common.Release, error)
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

func ImportMods(ctx context.Context, callback func(context.Context) error) error {
	return db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(localModsBucket)

		// Remove existing entries
		err := bucket.ForEach(func(k, _ []byte) error {
			err := bucket.Delete(k)
			if err != nil {
				return eris.Wrapf(err, "failed to delete key %v in local mod storage", k)
			}

			return nil
		})
		if err != nil {
			return err
		}

		// Clear indexes to make sure nothing uses them during the import since they'd be empty during the initial import.
		localVersionIdx.Clear()
		localTypeIdx.Clear()

		// Call the actual import function
		return callback(CtxWithTx(ctx, tx))
	})
}

func ImportUserSettings(ctx context.Context, callback func(context.Context, func(string, string, *client.UserSettings) error) error) error {
	return db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(userModSettingsBucket)

		// Remove existing entries
		err := bucket.ForEach(func(k, _ []byte) error {
			err := bucket.Delete(k)
			if err != nil {
				return eris.Wrapf(err, "failed to delete key %v in user settings", k)
			}

			return nil
		})
		if err != nil {
			return err
		}

		ctx = CtxWithTx(ctx, tx)

		return callback(ctx, func(modID, version string, us *client.UserSettings) error {
			encoded, err := proto.Marshal(us)
			if err != nil {
				return eris.Wrapf(err, "failed to serialise user settings for mod %s %s", modID, version)
			}

			err = bucket.Put([]byte(modID+"#"+version), encoded)
			if err != nil {
				return eris.Wrapf(err, "failed to save user settings for mod %s %s", modID, version)
			}

			return nil
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
			return eris.Wrapf(err, "failed to serialise mod %s", mod.Modid)
		}

		err = bucket.Put([]byte(mod.Modid), encoded)
		if err != nil {
			return eris.Wrapf(err, "failed to save mod %s", mod.Modid)
		}

		return nil
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
		return eris.Wrapf(err, "failed to encode release %s %s", release.Modid, release.Version)
	}

	err = bucket.Put([]byte(release.Modid+"#"+release.Version), encoded)
	if err != nil {
		return eris.Wrapf(err, "failed to save release %s %s", release.Modid, release.Version)
	}

	files := append([]*common.FileRef{release.Banner, release.Teaser}, release.Screenshots...)
	for _, item := range files {
		if item != nil {
			err = ImportFile(ctx, item)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func DeleteLocalModRelease(ctx context.Context, release *common.Release) error {
	tx := TxFromCtx(ctx)
	if tx == nil {
		return BatchUpdate(ctx, func(ctx context.Context) error {
			return DeleteLocalModRelease(ctx, release)
		})
	}

	importMutex.Lock()
	defer importMutex.Unlock()

	bucket := tx.Bucket(localModsBucket)
	err := bucket.Delete([]byte(release.Modid + "#" + release.Version))
	if err != nil {
		return eris.Wrapf(err, "failed to delete release %s %s", release.Modid, release.Version)
	}

	err = localVersionIdx.Remove(tx, release.Modid, release.Version)
	if err != nil {
		return eris.Wrapf(err, "failed to remove release %s %s from version index", release.Modid, release.Version)
	}

	return nil
}

func (p genericModProvider) GetMods(ctx context.Context) ([]*common.Release, error) {
	var result []*common.Release

	err := view(ctx, func(tx *bolt.Tx) error {
		// Retrieve IDs and the latest version for all known local mods
		bucket := tx.Bucket(p.bucket)
		result = make([]*common.Release, 0)

		return p.versionIndex.ForEach(func(modID string, versions []string) error {
			if len(versions) < 1 {
				return nil
			}

			item := bucket.Get([]byte(modID + "#" + versions[len(versions)-1]))
			if item == nil {
				return eris.Errorf("Failed to find mod %s from index", modID+"#"+versions[len(versions)-1])
			}

			meta := new(common.Release)
			err := proto.Unmarshal(item, meta)
			if err != nil {
				return eris.Wrapf(err, "failed to deserialise release %s %s", modID, versions[len(versions)-1])
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

func (p genericModProvider) GetAllReleases(ctx context.Context) ([]*common.Release, error) {
	result := make([]*common.Release, 0)

	err := db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(p.bucket)
		return bucket.ForEach(func(k, v []byte) error {
			if bytes.Contains(k, []byte("#")) {
				var rel common.Release
				err := proto.Unmarshal(v, &rel)
				if err != nil {
					return eris.Wrapf(err, "failed to deserialise release %s", k)
				}

				result = append(result, &rel)
			}
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
			return eris.Errorf("mod %s not found", id)
		}

		err := proto.Unmarshal(encoded, &mod)
		if err != nil {
			return eris.Wrapf(err, "failed to deserialise mod %s", id)
		}

		return nil
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
			return eris.Errorf("mod %s %s not found", id, version)
		}

		err := proto.Unmarshal(item, mod)
		if err != nil {
			return eris.Wrapf(err, "failed to deserialise release %s %s", id, version)
		}

		return nil
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

	// Protect against changes
	resultCopy := make([]string, len(result))
	copy(resultCopy, result)
	return resultCopy, nil
}

var _ ModProvider = (*genericModProvider)(nil)

func SaveUserSettingsForMod(ctx context.Context, id, version string, settings *client.UserSettings) error {
	return db.Update(func(tx *bolt.Tx) error {
		encoded, err := proto.Marshal(settings)
		if err != nil {
			return eris.Wrapf(err, "failed to serialise user settings for mod %s %s", id, version)
		}

		bucket := tx.Bucket(userModSettingsBucket)
		err = bucket.Put([]byte(id+"#"+version), encoded)
		if err != nil {
			return eris.Wrapf(err, "failed to save user settings for mod %s %s", id, version)
		}

		return nil
	})
}

func GetUserSettingsForMod(ctx context.Context, id, version string) (*client.UserSettings, error) {
	result := new(client.UserSettings)
	err := db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(userModSettingsBucket)

		encoded := bucket.Get([]byte(id + "#" + version))
		if encoded != nil {
			err := proto.Unmarshal(encoded, result)
			if err != nil {
				return eris.Wrapf(err, "failed to deserialise user settings for mod %s %s", id, version)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}
