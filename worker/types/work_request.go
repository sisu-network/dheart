package types

import (
	"errors"
	"fmt"

	"github.com/sisu-network/lib/log"
	"github.com/sisu-network/tss-lib/ecdsa/keygen"
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
	KeygenInput *keygen.LocalPreParams
	Threshold   int

	// Used for presign
	PresignInput *keygen.LocalPartySaveData

	// Used for signing
	Messages []string // TODO: Make this a byte array
	Chains   []string
}

func NewKeygenRequest(keyType, workId string, PIDs tss.SortedPartyIDs, threshold int, keygenInput *keygen.LocalPreParams) *WorkRequest {
	// Note: we only support ecdsa for now
	n := len(PIDs)
	request := baseRequest(EcKeygen, workId, n, PIDs, 1)
	request.KeygenInput = keygenInput
	request.Threshold = threshold
	request.KeygenType = keyType

	return request
}

func NewPresignRequest(workId string, PIDs tss.SortedPartyIDs, threshold int, presignInputs *keygen.LocalPartySaveData, forcedPresign bool, batchSize int) *WorkRequest {
	n := len(PIDs)

	request := baseRequest(EcPresign, workId, n, PIDs, batchSize)
	request.PresignInput = presignInputs
	request.Threshold = threshold
	request.ForcedPresign = forcedPresign

	return request
}

// the presignInputs param is optional
func NewSigningRequest(workId string, PIDs tss.SortedPartyIDs, threshold int, messages []string, chains []string, presignInput *keygen.LocalPartySaveData) *WorkRequest {
	n := len(PIDs)
	request := baseRequest(EcSigning, workId, n, PIDs, len(messages))
	request.PresignInput = presignInput
	request.Messages = messages
	request.Chains = chains
	request.Threshold = threshold

	return request
}

func baseRequest(workType WorkType, workdId string, n int, pIDs tss.SortedPartyIDs, batchSize int) *WorkRequest {
	return &WorkRequest{
		AllParties: pIDs,
		WorkType:   workType,
		WorkId:     workdId,
		BatchSize:  batchSize,
		N:          n,
	}
}

func (request *WorkRequest) Validate() error {
	switch request.WorkType {
	case EcKeygen:
	case EcPresign:
		if request.PresignInput == nil {
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
