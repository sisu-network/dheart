package types

import (
	"errors"
	"fmt"

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
	EcPresignInput *eckeygen.LocalPartySaveData

	// Eddsa
	EdSigningInput *edkeygen.LocalPartySaveData

	// Used for signing
	Messages []string // TODO: Make this a byte array
	Chains   []string
}

func NewEcKeygenRequest(keyType, workId string, pIds tss.SortedPartyIDs, threshold int, keygenInput *keygen.LocalPreParams) *WorkRequest {
	request := baseRequest(EcKeygen, workId, len(pIds), threshold, pIds, 1)
	request.EcKeygenInput = keygenInput
	request.KeygenType = keyType

	return request
}

func NewEcPresignRequest(workId string, pIds tss.SortedPartyIDs, threshold int, presignInputs *keygen.LocalPartySaveData, forcedPresign bool, batchSize int) *WorkRequest {
	request := baseRequest(EcPresign, workId, len(pIds), threshold, pIds, batchSize)
	request.EcPresignInput = presignInputs
	request.ForcedPresign = forcedPresign

	return request
}

// the presignInputs param is optional
func NewEcSigningRequest(workId string, pIds tss.SortedPartyIDs, threshold int, messages []string, chains []string, presignInput *keygen.LocalPartySaveData) *WorkRequest {
	n := len(pIds)
	request := baseRequest(EcSigning, workId, n, threshold, pIds, len(messages))
	request.EcPresignInput = presignInput
	request.Messages = messages
	request.Chains = chains

	return request
}

func NewEdKeygenRequest(keyType, workId string, pIds tss.SortedPartyIDs, threshold int) *WorkRequest {
	request := baseRequest(EdKeygen, workId, len(pIds), threshold, pIds, 1)
	request.KeygenType = keyType

	return request
}

func NewEdSigningRequest(workId string, pIds tss.SortedPartyIDs, threshold int, messages []string, chains []string, inputs *edkeygen.LocalPartySaveData, batchSize int) *WorkRequest {
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
	case EcPresign:
		if request.EcPresignInput == nil {
			return errors.New("Presign input could not be nil for presign task")
		}
	case EcSigning:
		if len(request.Messages) == 0 {
			return errors.New("Signing messages array could not be empty")
		}
		for i, msg := range request.Messages {
			if len(msg) == 0 {
				return fmt.Errorf("Message %d is empty", i)
			}
		}

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

	if request.WorkType == EcPresign {
		if request.ForcedPresign {
			// Force presign
			return 90
		}

		// Presign
		return 40
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

func (request *WorkRequest) IsPresign() bool {
	return request.WorkType == EcPresign
}

func (request *WorkRequest) IsSigning() bool {
	return request.WorkType == EcSigning || request.WorkType == EdSigning
}

func (request *WorkRequest) IsEcdsa() bool {
	return request.WorkType == EcKeygen || request.WorkType == EcPresign || request.WorkType == EcSigning
}

func (request *WorkRequest) IsEddsa() bool {
	return request.WorkType == EdKeygen || request.WorkType == EdSigning
}
