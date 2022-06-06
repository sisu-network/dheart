package db

import (
	p2ptypes "github.com/sisu-network/dheart/p2p/types"
	"github.com/sisu-network/tss-lib/ecdsa/keygen"

	ecsigning "github.com/sisu-network/tss-lib/ecdsa/signing"
	"github.com/sisu-network/tss-lib/tss"
)

//---/

type MockDatabase struct {
	// TODO: remove this unused variable
	ecSigningOneRound []*ecsigning.SignatureData_OneRoundData

	GetAvailablePresignShortFormFunc func() ([]string, []string, error)
	LoadPresignFunc                  func(presignIds []string) ([]*ecsigning.SignatureData_OneRoundData, error)
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

func (m *MockDatabase) SaveEcKeygen(chain string, workId string, pids []*tss.PartyID, keygenOutput []*keygen.LocalPartySaveData) error {
	return nil
}

func (m *MockDatabase) SavePresignData(workId string, pids []*tss.PartyID, presignOutputs []*ecsigning.SignatureData_OneRoundData) error {
	return nil
}

func (m *MockDatabase) GetAvailablePresignShortForm() ([]string, []string, error) {
	if m.GetAvailablePresignShortFormFunc != nil {
		return m.GetAvailablePresignShortFormFunc()
	}

	return []string{}, []string{}, nil
}

func (m *MockDatabase) LoadPresign(presignIds []string) ([]*ecsigning.SignatureData_OneRoundData, error) {
	if m.LoadPresignFunc != nil {
		return m.LoadPresignFunc(presignIds)
	}

	return nil, nil
}

func (m *MockDatabase) LoadPresignStatus(presignIds []string) ([]string, error) {
	return nil, nil
}

func (m *MockDatabase) LoadEcKeygen(chain string) (*keygen.LocalPartySaveData, error) {
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
