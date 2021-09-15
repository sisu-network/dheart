package mock

type MockStore struct {
	GetEncryptedFunc func(key []byte) (value []byte, err error)
	PutEncryptedFunc func(key, value []byte) error
}

func (m *MockStore) GetEncrypted(key []byte) (value []byte, err error) {
	if m.GetEncryptedFunc != nil {
		return m.GetEncryptedFunc(key)
	}
	return nil, nil

}

func (m *MockStore) PutEncrypted(key, value []byte) error {
	if m.PutEncryptedFunc != nil {
		return m.PutEncryptedFunc(key, value)
	}
	return nil
}
