package store

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/iterator"
)

type Store struct {
	db           *leveldb.DB
	encryptedKey []byte
	aesKey       []byte
}

func NewStore(path string, aesKey []byte) (*Store, error) {
	db, err := leveldb.OpenFile(path, nil)
	if err != nil {
		return nil, err
	}

	return &Store{
		db:     db,
		aesKey: aesKey,
	}, err
}

func (s *Store) Put(key, value []byte) error {
	return s.db.Put(key, value, nil)
}

func (s *Store) PutEncrypted(key, value []byte) error {
	blockCipher, err := aes.NewCipher(s.aesKey)
	if err != nil {
		return err
	}

	gcm, err := cipher.NewGCM(blockCipher)
	if err != nil {
		return err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return err
	}

	encrypted := gcm.Seal(nonce, nonce, value, nil)
	return s.Put(key, encrypted)
}

func (s *Store) Get(key []byte) (value []byte, err error) {
	return s.db.Get(key, nil)
}

func (s *Store) GetEncrypted(key []byte) (value []byte, err error) {
	encryptedValue, err := s.Get(key)
	if err != nil {
		return nil, err
	}

	blockCipher, err := aes.NewCipher(s.aesKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(blockCipher)
	if err != nil {
		return nil, err
	}

	nonce, ciphertext := encryptedValue[:gcm.NonceSize()], encryptedValue[gcm.NonceSize():]
	data, err := gcm.Open(nil, nonce, ciphertext, nil)

	return data, err
}

func (s *Store) Iterator() iterator.Iterator {
	return s.db.NewIterator(nil, nil)
}
