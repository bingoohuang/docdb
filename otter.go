package main

import (
	"fmt"
	"github.com/cockroachdb/pebble"
	"github.com/maypok86/otter"
	"io"
)

type OtterCache struct {
	Cache otter.Cache[string, string]
}

func (o OtterCache) Close() error {
	o.Cache.Close()
	return nil
}

func (o *OtterCache) Open(string) error {
	// create a cache with capacity equal to 10000 elements
	var err error
	o.Cache, err = otter.MustBuilder[string, string](60_000).
		// CollectStats().
		// Cost(func(key, value string) uint32 {
		// 	return 1
		// }).
		// WithTTL(time.Hour).
		Build()
	if err != nil {
		return fmt.Errorf("create otter: %w", err)
	}

	return nil
}

func (o OtterCache) GetIndex(key []byte) ([]byte, io.Closer, error) {
	val, ok := o.Cache.Get("_index_" + string(key))
	if !ok {
		return nil, o, pebble.ErrNotFound
	}

	return []byte(val), nil, nil
}

func (o OtterCache) SetIndex(key, val []byte) error {
	ok := o.Cache.Set("_index_"+string(key), string(val))
	if !ok {
		return fmt.Errorf("key-value pair had too much cost and the Set was dropped")
	}
	return nil
}

func (o OtterCache) GetVal(key []byte) ([]byte, io.Closer, error) {
	val, ok := o.Cache.Get(string(key))
	if !ok {
		return nil, o, pebble.ErrNotFound
	}

	return []byte(val), nil, nil
}

func (o OtterCache) SetVal(key, val []byte) error {
	ok := o.Cache.Set(string(key), string(val))
	if !ok {
		return fmt.Errorf("key-value pair had too much cost and the Set was dropped")
	}
	return nil
}

func (o OtterCache) Walk(walker func(key, val []byte) error) error {
	o.Cache.Range(func(key, value string) bool {
		if err := walker([]byte(key), []byte(value)); err != nil {
			return false
		}

		return true
	})

	return nil
}

func (o OtterCache) Flush() {}

var _ DB = (*OtterCache)(nil)
