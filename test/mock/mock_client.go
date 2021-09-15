package mock

import (
	"github.com/sisu-network/dheart/types"
)

type MockClient struct {
	TryDialFunc                func()
	PostKeygenResultFunc       func(workId string)
	BroadcastKeygenResultFunc  func(chain string, pubKeyBytes []byte, address string) error
	BroadcastKeySignResultFunc func(result *types.KeysignResult) error
}

func (m *MockClient) TryDial() {
	if m.TryDialFunc != nil {
		m.TryDialFunc()
	}
}

func (m *MockClient) PostKeygenResult(workId string) {
	if m.PostKeygenResultFunc != nil {
		m.PostKeygenResultFunc(workId)
	}
}

func (m *MockClient) BroadcastKeygenResult(chain string, pubKeyBytes []byte, address string) error {
	if m.BroadcastKeygenResultFunc != nil {
		return m.BroadcastKeygenResultFunc(chain, pubKeyBytes, address)
	}

	return nil
}

func (m *MockClient) BroadcastKeySignResult(result *types.KeysignResult) error {
	if m.BroadcastKeySignResultFunc != nil {
		return m.BroadcastKeySignResultFunc(result)
	}

	return nil
}
