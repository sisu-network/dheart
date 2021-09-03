package mock

import "github.com/sisu-network/dheart/client"

type MockClient struct {
	tryDialFunc          func()
	postKeygenResultFunc func(workId string)
}

func NewClient(
	tryDialFunc func(),
	postKeygenResultFunc func(workId string),
) client.Client {
	return &MockClient{
		tryDialFunc,
		postKeygenResultFunc,
	}
}

func (m *MockClient) TryDial() {
	if m.tryDialFunc != nil {
		m.tryDialFunc()
	}
}

func (m *MockClient) PostKeygenResult(workId string) {
	if m.postKeygenResultFunc != nil {
		m.postKeygenResultFunc(workId)
	}
}
