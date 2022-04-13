package main

import (
	"github.com/akrylysov/pogreb"
	"go.uber.org/multierr"
	"io"
	"log"
)

type pogrebDB struct {
	path    string
	db      *pogreb.DB
	indexDb *pogreb.DB
}

func (s pogrebDB) Close() error { return multierr.Append(s.db.Close(), s.indexDb.Close()) }

func (s *pogrebDB) Open(path string) (err error) {
	s.path = path
	if s.db, err = pogreb.Open(path, nil); err != nil {
		return err
	}
	if s.indexDb, err = pogreb.Open(path+".index", nil); err != nil {
		return err
	}
	return nil
}

func (s pogrebDB) GetIndex(key []byte) (v []byte, c io.Closer, err error) {
	v, err = s.indexDb.Get(key)
	return
}

func (s pogrebDB) SetIndex(key, val []byte) error { return s.indexDb.Put(key, val) }

func (s pogrebDB) GetVal(key []byte) (v []byte, c io.Closer, err error) {
	v, err = s.db.Get(key)
	return
}

func (s pogrebDB) SetVal(key, val []byte) error { return s.db.Put(key, val) }

func (s pogrebDB) Walk(walker func(key []byte, val []byte) error) error {
	for it := s.db.Items(); ; {
		key, val, err := it.Next()
		if err == pogreb.ErrIterationDone {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		if err := walker(key, val); err != nil {
			return err
		}
	}
	return nil
}

func (s pogrebDB) Flush() {
	log.Printf("flush db result %v", logErr(s.db.Sync()))
	log.Printf("flush index result %v", logErr(s.indexDb.Sync()))
}
