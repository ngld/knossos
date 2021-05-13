package helpers

import (
	"sort"

	"github.com/ngld/knossos/packages/api/client"
)

type SimpleModListItemsByTitle []*client.SimpleModList_Item

var _ sort.Interface = (*SimpleModListItemsByTitle)(nil)

func (s SimpleModListItemsByTitle) Len() int           { return len(s) }
func (s SimpleModListItemsByTitle) Less(i, j int) bool { return s[i].Title < s[j].Title }
func (s SimpleModListItemsByTitle) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
