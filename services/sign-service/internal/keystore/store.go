package keystore

import (
	"encoding/hex"

	"github.com/syndtr/goleveldb/leveldb"
)

type levelStore struct {
	db *leveldb.DB
}

func newLevelStore(path string) (*levelStore, error) {
	handle, err := leveldb.OpenFile(path, nil)
	if err != nil {
		return nil, err
	}
	return &levelStore{db: handle}, nil
}

func (db *levelStore) Put(key []byte, value []byte) error {
	return db.db.Put(key, value, nil)
}

func (db *levelStore) Get(key []byte) ([]byte, error) {
	return db.db.Get(key, nil)
}

func (db *levelStore) Close() error {
	return db.db.Close()
}

func toBytes(dataStr string) []byte {
	dataBytes, _ := hex.DecodeString(dataStr)
	return dataBytes
}

func toString(byteArr []byte) string {
	return hex.EncodeToString(byteArr)
}
