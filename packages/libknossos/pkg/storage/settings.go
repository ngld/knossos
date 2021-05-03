package storage

import (
	"context"
	"encoding/json"

	bolt "go.etcd.io/bbolt"

	"github.com/ngld/knossos/packages/api/client"
)

var settingsBucket = []byte("settings")

func GetSettings(ctx context.Context) (*client.Settings, error) {
	settings := new(client.Settings)
	err := db.View(func(tx *bolt.Tx) error {
		item := tx.Bucket(settingsBucket).Get([]byte("settings"))
		if item == nil {
			return nil
		}

		return json.Unmarshal(item, &settings)
	})
	return settings, err
}

func SaveSettings(ctx context.Context, settings *client.Settings) error {
	encoded, err := json.Marshal(settings)
	if err != nil {
		return err
	}

	return db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(settingsBucket).Put([]byte("settings"), encoded)
	})
}
