package main

import (
	"errors"
	"io"

	"github.com/nalgeon/redka"
	_ "modernc.org/sqlite"
)

type RedkaDB struct {
	DB *redka.DB
}

func (r RedkaDB) Close() error {
	return r.DB.Close()
}

func (r *RedkaDB) Open(path string) error {
	// modernc.org/sqlite uses a different driver name
	// ("sqlite" instead of "sqlite3").
	opts := redka.Options{
		DriverName: "sqlite",
	}

	db, err := redka.Open(path+"-redka.db", &opts)
	if err != nil {
		return err
	}
	r.DB = db
	return nil
}

func (r RedkaDB) GetIndex(key []byte) ([]byte, io.Closer, error) {
	val, err := r.DB.Str().Get("idx:" + string(key))
	if errors.Is(err, redka.ErrNotFound) {
		return nil, nil, ErrNotFound
	}

	return val, nil, err
}

func (r RedkaDB) SetIndex(key, val []byte) error {
	return r.DB.Str().Set("idx:"+string(key), string(val))
}

func (r RedkaDB) GetVal(key []byte) ([]byte, io.Closer, error) {
	val, err := r.DB.Str().Get("val:" + string(key))
	if errors.Is(err, redka.ErrNotFound) {
		return nil, nil, ErrNotFound
	}

	return val, nil, err
}

func (r RedkaDB) SetVal(key, val []byte) error {
	return r.DB.Str().Set("val:"+string(key), string(val))
}

func (r RedkaDB) Walk(walker func(key []byte, val []byte) error) error {
	return nil
}

func (r RedkaDB) Flush() {
}

var _ DB = (*RedkaDB)(nil)
