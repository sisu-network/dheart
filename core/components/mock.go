package components

import (
	"github.com/sisu-network/tss-lib/ecdsa/presign"
	"github.com/sisu-network/tss-lib/tss"
)

//---/

type MockAvailablePresigns struct {
	LoadFunc                 func() error
	GetAvailablePresignsFunc func(batchSize int, n int, allPids map[string]*tss.PartyID) ([]string, []*tss.PartyID)
	AddPresignFunc           func(workId string, partyIds []*tss.PartyID, presignOutputs []*presign.LocalPresignData)
}

func NewMockAvailablePresigns() AvailablePresigns {
	return &MockAvailablePresigns{}
}

func (m *MockAvailablePresigns) Load() error {
	if m.LoadFunc != nil {
		return m.LoadFunc()
	}

	return nil
}

func (m *MockAvailablePresigns) GetAvailablePresigns(batchSize int, n int, allPids map[string]*tss.PartyID) ([]string, []*tss.PartyID) {
	if m.GetAvailablePresignsFunc != nil {
		m.GetAvailablePresigns(batchSize, n, allPids)
	}

	return nil, nil
}

func (m *MockAvailablePresigns) AddPresign(workId string, partyIds []*tss.PartyID, presignOutputs []*presign.LocalPresignData) {
	if m.AddPresignFunc != nil {
		m.AddPresignFunc(workId, partyIds, presignOutputs)
	}
}
