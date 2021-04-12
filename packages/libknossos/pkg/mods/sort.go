package mods

import "github.com/ngld/knossos/packages/api/common"

type ReleaseCollection []*common.Release

func (c ReleaseCollection) Len() int      { return len(c) }
func (c ReleaseCollection) Swap(i, j int) { c[i], c[j] = c[j], c[i] }

type SortByTitle struct{ ReleaseCollection }

func (c SortByTitle) Less(i, j int) bool {
	return c.ReleaseCollection[i].GetTitle() < c.ReleaseCollection[j].GetTitle()
}
