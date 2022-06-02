package types

import (
	"errors"

	"github.com/sisu-network/lib/log"
	"github.com/sisu-network/tss-lib/ecdsa/keygen"
	eckeygen "github.com/sisu-network/tss-lib/ecdsa/keygen"
	edkeygen "github.com/sisu-network/tss-lib/eddsa/keygen"
	"github.com/sisu-network/tss-lib/tss"
)

type WorkRequest struct {
	WorkType      WorkType
	AllParties    []*tss.PartyID
	WorkId        string
	N             int // The number of available participants required to do this task.
	ForcedPresign bool
	BatchSize     int

	// Used only for keygen, presign & signing
	KeygenType  string
	KeygenIndex int
	Threshold   int

	// Used for ecdsa
	EcKeygenInput  *eckeygen.LocalPreParams
	EcSigningInput *eckeygen.LocalPartySaveData

	// Eddsa
	EdSigningInput *edkeygen.LocalPartySaveData

	// Used for signing
	Messages [][]byte // TODO: Make this a byte array
	Chains   []string
}

func NewEcKeygenRequest(keyType, workId string, pIds tss.SortedPartyIDs, threshold int, keygenInput *keygen.LocalPreParams) *WorkRequest {
	request := baseRequest(EcKeygen, workId, len(pIds), threshold, pIds, 1)
	request.EcKeygenInput = keygenInput
	request.KeygenType = keyType

	return request
}

// the presignInputs param is optional
func NewEcSigningRequest(workId string, pIds tss.SortedPartyIDs, threshold int, messages [][]byte, chains []string,
	keygenOutput *keygen.LocalPartySaveData) *WorkRequest {
	n := len(pIds)
	request := baseRequest(EcSigning, workId, n, threshold, pIds, len(messages))
	request.EcSigningInput = keygenOutput
	request.Messages = messages
	request.Chains = chains

	return request
}

func NewEdKeygenRequest(keyType, workId string, pIds tss.SortedPartyIDs, threshold int) *WorkRequest {
	request := baseRequest(EdKeygen, workId, len(pIds), threshold, pIds, 1)
	request.KeygenType = keyType

	return request
}

func NewEdSigningRequest(workId string, pIds tss.SortedPartyIDs, threshold int, messages [][]byte,
	chains []string, inputs *edkeygen.LocalPartySaveData, batchSize int) *WorkRequest {
	request := baseRequest(EdSigning, workId, len(pIds), threshold, pIds, batchSize)
	request.EdSigningInput = inputs
	request.Messages = messages
	request.Chains = chains

	return request
}

func baseRequest(workType WorkType, workdId string, n int, threshold int, pIDs tss.SortedPartyIDs, batchSize int) *WorkRequest {
	return &WorkRequest{
		AllParties: pIDs,
		WorkType:   workType,
		WorkId:     workdId,
		BatchSize:  batchSize,
		N:          n,
		Threshold:  threshold,
	}
}

func (request *WorkRequest) Validate() error {
	switch request.WorkType {
	case EcKeygen:
	case EdKeygen:
	case EdSigning:
	default:
		return errors.New("Invalid request type")
	}
	return nil
}

// GetMinPartyCount returns the minimum number of parties needed to do this job.
func (request *WorkRequest) GetMinPartyCount() int {
	if request.IsKeygen() {
		return request.N
	}

	return request.Threshold + 1
}

func (request *WorkRequest) GetPriority() int {
	// Keygen
	if request.WorkType == EcKeygen || request.WorkType == EdKeygen {
		return 100
	}

	// Signing
	if request.WorkType == EcSigning || request.WorkType == EdSigning {
		return 80
	}

	log.Critical("Unknown work type", request.WorkType)

	return -1
}

func (request *WorkRequest) IsKeygen() bool {
	return request.WorkType == EcKeygen || request.WorkType == EdKeygen
}

func (request *WorkRequest) IsSigning() bool {
	return request.WorkType == EcSigning || request.WorkType == EdSigning
}

func (request *WorkRequest) IsEcPresign() bool {
	return request.WorkType == EcSigning && request.Messages[0] == nil
}

func (request *WorkRequest) IsEcdsa() bool {
	return request.WorkType == EcKeygen || request.WorkType == EcSigning
}

func (request *WorkRequest) IsEddsa() bool {
	return request.WorkType == EdKeygen || request.WorkType == EdSigning
}
