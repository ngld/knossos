package storage

import (
	"encoding/json"
	"sort"

	bolt "go.etcd.io/bbolt"
)

var indexBucket = []byte("_indexes")

type StringListSorter func(string, []string) error

type StringListIndex struct {
	cache     map[string][]string
	sorter    StringListSorter
	Name      string
	batchMode bool
}

func NewStringListIndex(name string, sorter StringListSorter) *StringListIndex {
	if sorter == nil {
		sorter = func(_ string, slice []string) error {
			sort.Strings(slice)
			return nil
		}
	}

	return &StringListIndex{
		Name:   name,
		sorter: sorter,
	}
}

func (i *StringListIndex) Open(tx *bolt.Tx) error {
	bucket := tx.Bucket(indexBucket)
	data := bucket.Get([]byte(i.Name))
	if data != nil {
		err := json.Unmarshal(data, &i.cache)
		if err != nil {
			return err
		}
	}

	return nil
}

func (i *StringListIndex) StartBatch() {
	i.batchMode = true
}

func (i *StringListIndex) FinishBatch(tx *bolt.Tx) error {
	i.batchMode = false
	return i.Save(tx)
}

func (i *StringListIndex) Lookup(key string) []string {
	return i.cache[key]
}

func (i *StringListIndex) ForEach(cb func(string, []string) error) error {
	for k, v := range i.cache {
		err := cb(k, v)
		if err != nil {
			return err
		}
	}

	return nil
}

func (i *StringListIndex) Clear() {
	i.cache = make(map[string][]string)
}

func (i *StringListIndex) BatchedAdd(key, value string) {
	if !i.batchMode {
		panic("BatchedAdd() called on an index that wasn't in batch mode!")
	}

	if i.cache == nil {
		i.Clear()
	}
	i.cache[key] = append(i.cache[key], value)
}

func (i *StringListIndex) BatchedRemove(key, value string) {
	if !i.batchMode {
		panic("BatchedAdd() called on an index that wasn't in batch mode!")
	}

	if i.cache == nil {
		return
	}

	for idx, item := range i.cache[key] {
		if item == value {
			i.cache[key] = append(i.cache[key][:idx], i.cache[key][idx+1:]...)
			break
		}
	}
}

func (i *StringListIndex) BatchedRemoveAll(key string) {
	if !i.batchMode {
		panic("BatchedAdd() called on an index that wasn't in batch mode!")
	}

	if i.cache == nil {
		return
	}

	delete(i.cache, key)
}

func (i *StringListIndex) Add(tx *bolt.Tx, key, value string) error {
	if i.cache == nil {
		i.Clear()
	}
	i.cache[key] = append(i.cache[key], value)

	if !i.batchMode {
		return i.Save(tx)
	}

	return nil
}

func (i *StringListIndex) Remove(tx *bolt.Tx, key, value string) error {
	if i.cache == nil {
		i.Clear()
	}

	for idx, item := range i.cache[key] {
		if item == value {
			i.cache[key] = append(i.cache[key][:idx], i.cache[key][idx+1:]...)
			break
		}
	}

	if !i.batchMode {
		return i.Save(tx)
	}

	return nil
}

func (i *StringListIndex) RemoveAll(tx *bolt.Tx, key, value string) error {
	if i.cache == nil {
		i.Clear()
	}

	delete(i.cache, key)

	if !i.batchMode {
		return i.Save(tx)
	}

	return nil
}

func (i *StringListIndex) Save(tx *bolt.Tx) error {
	if i.cache == nil {
		i.Clear()
	}

	err := i.ForEach(i.sorter)
	if err != nil {
		return nil
	}

	// Remove duplicates
	for k, v := range i.cache {
		if len(v) > 0 {
			last := v[len(v)-1]
			for idx := len(v) - 2; idx >= 0; idx-- {
				if v[idx] == last {
					v = append(v[:idx], v[idx+1:]...)
				} else {
					last = v[idx]
				}
			}

			i.cache[k] = v
		}
	}

	encoded, err := json.Marshal(i.cache)
	if err != nil {
		return err
	}

	bucket := tx.Bucket(indexBucket)
	return bucket.Put([]byte(i.Name), encoded)
}
