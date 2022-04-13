package main

import (
	"github.com/flower-corp/lotusdb"
	"go.uber.org/multierr"
	"io"
	"log"
)

type lotusdbDB struct {
	path    string
	db      *lotusdb.LotusDB // Primary data
	indexDb *lotusdb.LotusDB // Index data
}

// Walk implements DB
func (s *lotusdbDB) Walk(walker func(key []byte, val []byte) error) error {
	return nil
}

// GetVal implements DB
func (s *lotusdbDB) GetVal(key []byte) (val []byte, closer io.Closer, err error) {
	val, err = s.db.Get(key)
	return
}

// SetVal implements DB
func (s *lotusdbDB) SetVal(key []byte, val []byte) error { return s.db.Put(key, val) }

// SetIndex implements DB
func (s *lotusdbDB) SetIndex(key, val []byte) error { return s.indexDb.Put(key, val) }

// GetIndex implements DB
func (s *lotusdbDB) GetIndex(key []byte) (val []byte, closer io.Closer, err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("recover from key %s", key)
		}
	}()

	val, err = s.indexDb.Get(key)
	return
}

// Close implements DB
func (s *lotusdbDB) Close() error { return multierr.Append(s.db.Close(), s.indexDb.Close()) }

// Flush implements DB
func (s *lotusdbDB) Flush() {
	s.flush(s.db, s.path)
	s.flush(s.indexDb, s.path+".index")
}

func (s *lotusdbDB) flush(db *lotusdb.LotusDB, path string) {
	cfOpts := lotusdb.DefaultOptions(path).CfOpts
	cfOpts.CfName = lotusdb.DefaultColumnFamilyName
	cf, err := db.OpenColumnFamily(cfOpts)
	if err != nil {
		log.Printf("open column family failed: %v", err)
		return
	}
	log.Printf("sync db %s", logErr(cf.Sync()))
}

// Open implements DB
func (s *lotusdbDB) Open(path string) (err error) {
	s.path = path
	if s.db, err = lotusdb.Open(lotusdb.DefaultOptions(path)); err != nil {
		return err
	}
	if s.indexDb, err = lotusdb.Open(lotusdb.DefaultOptions(path + ".index")); err != nil {
		return err
	}

	return nil
}
