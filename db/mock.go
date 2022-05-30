package db

import (
	p2ptypes "github.com/sisu-network/dheart/p2p/types"
	"github.com/sisu-network/tss-lib/ecdsa/keygen"
	"github.com/sisu-network/tss-lib/ecdsa/presign"
	"github.com/sisu-network/tss-lib/tss"
)

//---/

type MockDatabase struct {
	// TODO: remove this unused variable
	signingInput []*presign.LocalPresignData

	GetAvailablePresignShortFormFunc func() ([]string, []string, error)
	LoadPresignFunc                  func(presignIds []string) ([]*presign.LocalPresignData, error)
}

func NewMockDatabase() Database {
	return &MockDatabase{}
}

func (m *MockDatabase) Init() error {
	return nil
}

func (m *MockDatabase) SavePreparams(preparams *keygen.LocalPreParams) error {
	return nil
}

func (m *MockDatabase) LoadPreparams() (*keygen.LocalPreParams, error) {
	return nil, nil
}

func (m *MockDatabase) SaveKeygenData(chain string, workId string, pids []*tss.PartyID, keygenOutput []*keygen.LocalPartySaveData) error {
	return nil
}

func (m *MockDatabase) SavePresignData(workId string, pids []*tss.PartyID, presignOutputs []*presign.LocalPresignData) error {
	return nil
}

func (m *MockDatabase) GetAvailablePresignShortForm() ([]string, []string, error) {
	if m.GetAvailablePresignShortFormFunc != nil {
		return m.GetAvailablePresignShortFormFunc()
	}

	return []string{}, []string{}, nil
}

func (m *MockDatabase) LoadPresign(presignIds []string) ([]*presign.LocalPresignData, error) {
	if m.LoadPresignFunc != nil {
		return m.LoadPresignFunc(presignIds)
	}

	return nil, nil
}

func (m *MockDatabase) LoadPresignStatus(presignIds []string) ([]string, error) {
	return nil, nil
}

func (m *MockDatabase) LoadKeygenData(chain string) (*keygen.LocalPartySaveData, error) {
	return nil, nil
}

func (m *MockDatabase) UpdatePresignStatus(presignIds []string) error {
	return nil
}

func (m *MockDatabase) SavePeers([]*p2ptypes.Peer) error {
	return nil
}

func (m *MockDatabase) LoadPeers() []*p2ptypes.Peer {
	return nil
}
