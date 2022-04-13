package main

import (
	"github.com/bingoohuang/gg/pkg/iox"
	"github.com/cockroachdb/pebble"
	"go.uber.org/multierr"
	"io"
	"log"
)

type pebbleDB struct {
	db      *pebble.DB // Primary data
	indexDb *pebble.DB // Index data
}

func (s *pebbleDB) Walk(walker func(key, val []byte) error) error {
	iter := s.db.NewIter(nil)
	defer iox.Close(iter)
	for iter.First(); iter.Valid(); iter.Next() {
		if err := walker(iter.Key(), iter.Value()); err != nil {
			return err
		}
	}
	return nil
}

// GetVal implements DB
func (s *pebbleDB) GetVal(key []byte) ([]byte, io.Closer, error) { return s.db.Get(key) }

// SetVal implements DB
func (s *pebbleDB) SetVal(key []byte, val []byte) error { return s.db.Set(key, val, pebble.NoSync) }

// SetIndex implements DB
func (s *pebbleDB) SetIndex(key, val []byte) error { return s.indexDb.Set(key, val, pebble.NoSync) }

// GetIndex implements DB
func (s *pebbleDB) GetIndex(key []byte) ([]byte, io.Closer, error) { return s.indexDb.Get(key) }

// Close implements DB
func (s *pebbleDB) Close() error { return multierr.Append(s.db.Close(), s.indexDb.Close()) }

// Flush implements DB
func (s *pebbleDB) Flush() {
	log.Printf("flush db result %v", logErr(s.db.Flush()))
	log.Printf("flush index result %v", logErr(s.indexDb.Flush()))
}

// Open implements DB
func (s *pebbleDB) Open(path string) (err error) {
	if s.db, err = pebble.Open(path, &pebble.Options{}); err != nil {
		return err
	}

	if s.indexDb, err = pebble.Open(path+".index", &pebble.Options{}); err != nil {
		return err
	}
	return nil
}
