package store

import (
	"crypto/rand"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func createStoreForTest(t *testing.T) *Store {
	aesKey := make([]byte, 16)
	rand.Read(aesKey)
	store, err := NewStore(".", aesKey)
	assert.Nil(t, err)

	return store
}

func TestPutGet(t *testing.T) {
	store := createStoreForTest(t)

	text := []byte("text")
	key := []byte("key")

	err := store.Put(key, text)
	assert.Nil(t, err)

	value, err := store.Get(key)
	assert.Equal(t, value, text, "Incorrect data")

	err = os.RemoveAll("./path")
	assert.Nil(t, err)
}

func TestEncrypted(t *testing.T) {
	store := createStoreForTest(t)

	text := []byte("text")
	key := []byte("key")

	err := store.PutEncrypted(key, text)
	assert.Nil(t, err)

	value, err := store.GetEncrypted(key)
	assert.Equal(t, value, text, "Incorrect data")

	err = os.RemoveAll("./path")
	assert.Nil(t, err)
}
