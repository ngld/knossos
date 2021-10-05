//go:build !windows

package platform

import "context"

func GetVoices(ctx context.Context) ([]string, error) {
	return []string{}, nil
}
