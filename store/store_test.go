package store

import (
	"crypto/rand"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func createStoreForTest(t *testing.T, path string) *DefaultStore {
	aesKey := make([]byte, 16)
	rand.Read(aesKey)
	store, err := NewStore(path, aesKey)
	assert.Nil(t, err)

	return store.(*DefaultStore)
}

func TestPutGet(t *testing.T) {
	path := "./leveldb"
	store := createStoreForTest(t, path)

	text := []byte("text")
	key := []byte("key")

	err := store.Put(key, text)
	assert.Nil(t, err)

	value, err := store.Get(key)
	assert.Equal(t, value, text, "Incorrect data")

	err = os.RemoveAll(path)
	assert.Nil(t, err)
}

func TestEncrypted(t *testing.T) {
	path := "./leveldb"
	store := createStoreForTest(t, path)

	text := []byte("text")
	key := []byte("key")

	err := store.PutEncrypted(key, text)
	assert.Nil(t, err)

	value, err := store.GetEncrypted(key)
	assert.Equal(t, value, text, "Incorrect data")

	err = os.RemoveAll(path)
	assert.Nil(t, err)
}
