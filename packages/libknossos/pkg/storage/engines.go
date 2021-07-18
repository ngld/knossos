package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"os"

	"github.com/rotisserie/eris"
	bolt "go.etcd.io/bbolt"
)

type JSONFlags struct {
	Version struct {
		Full        string
		Major       int
		Minor       int
		Build       int
		HasRevision bool `json:"has_revision"`
		Revision    int
		RevisionStr string `json:"revision_str"`
	}

	// easy_flags skipped

	Flags []struct {
		Name        string
		Description string
		FsoOnly     bool `json:"fso_only"`
		// on_flags and off_flags skipped
		Type   string
		WebURL string `json:"web_url"`
	}

	Caps     []string
	Voices   []string
	Displays []struct {
		Index  int
		Name   string
		X      int
		Y      int
		Width  int
		Height int
		Modes  []struct {
			X    int
			Y    int
			Bits int
		}
	}

	Openal struct {
		VersionMajor    int             `json:"version_major"`
		VersionMinor    int             `json:"version_minor"`
		DefaultPlayback string          `json:"default_playback"`
		DefaultCapture  string          `json:"default_capture"`
		PlaybackDevices []string        `json:"playback_devices"`
		CaptureDevices  []string        `json:"capture_devices"`
		EfxSupport      map[string]bool `json:"efx_support"`
	}

	Joysticks []struct {
		Name       string
		GUID       string
		NumAxes    int  `json:"num_axes"`
		NumBalls   int  `json:"num_balls"`
		NumButtons int  `json:"num_buttons"`
		NumHats    int  `json:"num_hats"`
		IsHaptic   bool `json:"is_haptic"`
	}

	PrefPath string `json:"pref_path"`
}

var engineFlagsBucket = []byte("engine-flags")

func SaveEngineFlags(ctx context.Context, path string, flags *JSONFlags) error {
	return update(ctx, func(tx *bolt.Tx) error {
		bucket := tx.Bucket(engineFlagsBucket)
		encoded, err := json.Marshal(flags)
		if err != nil {
			return err
		}

		return bucket.Put([]byte("file#"+path), encoded)
	})
}

func GetEngineFlags(ctx context.Context, path string) (*JSONFlags, error) {
	var flags *JSONFlags
	err := view(ctx, func(tx *bolt.Tx) error {
		bucket := tx.Bucket(engineFlagsBucket)
		encoded := bucket.Get([]byte("file#" + path))
		if encoded == nil {
			return nil
		}

		flags = new(JSONFlags)
		return json.Unmarshal(encoded, flags)
	})
	if err != nil {
		return nil, err
	}

	return flags, nil
}

func cleanEngineFlags(ctx context.Context, tx *bolt.Tx) error {
	bucket := tx.Bucket(engineFlagsBucket)
	filePrefix := []byte("file#")

	return bucket.ForEach(func(k, v []byte) error {
		if bytes.HasPrefix(k, filePrefix) {
			filePath := string(k[5:])
			_, err := os.Stat(filePath)
			if err == nil {
				return nil
			}
			if !eris.Is(err, os.ErrNotExist) {
				return eris.Wrapf(err, "failed to check %s", filePath)
			}

			return bucket.Delete(k)
		}

		return nil
	})
}
