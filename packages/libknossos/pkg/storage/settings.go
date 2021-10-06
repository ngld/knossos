package storage

import (
	"context"
	"encoding/json"

	bolt "go.etcd.io/bbolt"

	"github.com/ngld/knossos/packages/api/client"
	"github.com/rotisserie/eris"
)

var settingsBucket = []byte("settings")

func GetSettings(ctx context.Context) (*client.Settings, error) {
	settings := new(client.Settings)
	err := db.View(func(tx *bolt.Tx) error {
		item := tx.Bucket(settingsBucket).Get([]byte("settings"))
		if item == nil {
			return nil
		}

		err := json.Unmarshal(item, &settings)
		if err != nil {
			return eris.Wrap(err, "failed to deserialise settings")
		}

		return nil
	})
	return settings, err
}

func SaveSettings(ctx context.Context, settings *client.Settings) error {
	encoded, err := json.Marshal(settings)
	if err != nil {
		return eris.Wrap(err, "failed to serialise settings")
	}

	return db.Update(func(tx *bolt.Tx) error {
		err := tx.Bucket(settingsBucket).Put([]byte("settings"), encoded)
		if err != nil {
			return eris.Wrap(err, "failed to save settings")
		}

		return nil
	})
}
