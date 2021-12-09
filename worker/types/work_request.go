package types

import (
	"errors"

	"github.com/sisu-network/lib/log"
	"github.com/sisu-network/tss-lib/ecdsa/keygen"
	"github.com/sisu-network/tss-lib/tss"
)

type WorkRequest struct {
	Chain         string
	WorkType      WorkType
	AllParties    []*tss.PartyID
	WorkId        string
	N             int // The number of available participants required to do this task.
	ForcedPresign bool
	BatchSize     int

	// Used only for keygen, presign & signing
	KeygenType  string
	KeygenInput *keygen.LocalPreParams
	Threshold   int

	// Used for presign
	PresignInput *keygen.LocalPartySaveData

	// Used for signing
	Message string
}

func NewKeygenRequest(keyType, workId string, n int, PIDs tss.SortedPartyIDs, keygenInput *keygen.LocalPreParams, threshold int) *WorkRequest {
	// Note: we only support ecdsa for now
	request := baseRequest(EcdsaKeygen, workId, n, PIDs)
	request.KeygenInput = keygenInput
	request.Threshold = threshold
	request.KeygenType = keyType

	return request
}

func NewPresignRequest(workId string, n int, PIDs tss.SortedPartyIDs, presignInput keygen.LocalPartySaveData, forcedPresign bool) *WorkRequest {
	request := baseRequest(EcdsaPresign, workId, n, PIDs)
	request.PresignInput = &presignInput
	request.ForcedPresign = forcedPresign

	return request
}

func NewSigningRequets(chain, workId string, n int, PIDs tss.SortedPartyIDs, message string) *WorkRequest {
	request := baseRequest(EcdsaSigning, workId, n, PIDs)
	request.Chain = chain
	request.Message = message

	return request
}

func baseRequest(workType WorkType, workdId string, n int, pIDs tss.SortedPartyIDs) *WorkRequest {
	return &WorkRequest{
		AllParties: pIDs,
		WorkType:   workType,
		WorkId:     workdId,
		BatchSize:  1, // TODO: Support real batching
		N:          n,
	}
}

func (request *WorkRequest) Validate() error {
	switch request.WorkType {
	case EcdsaKeygen:
	case EcdsaPresign:
		if request.PresignInput == nil {
			return errors.New("Presign input could not be nil for presign task")
		}
	case EcdsaSigning:
		if request.PresignInput == nil {
			return errors.New("Presign input could not be nil for signing task")
		}
		if request.Message == "" {
			return errors.New("Signing message could not be empty")
		}

	case EddsaKeygen:
	case EddsaPresign:
	case EddsaSigning:
	default:
		return errors.New("Invalid request type")
	}
	return nil
}

func (request *WorkRequest) GetPriority() int {
	// Keygen
	if request.WorkType == EcdsaKeygen || request.WorkType == EddsaKeygen {
		return 100
	}

	if request.WorkType == EcdsaPresign || request.WorkType == EddsaPresign {
		if request.ForcedPresign {
			// Force presign
			return 90
		}

		// Presign
		return 70
	}

	// Signing
	if request.WorkType == EcdsaSigning || request.WorkType == EddsaSigning {
		return 60
	}

	log.Critical("Unknown work type", request.WorkType)

	return -1
}

func (request *WorkRequest) IsKeygen() bool {
	return request.WorkType == EcdsaKeygen || request.WorkType == EddsaKeygen
}

func (request *WorkRequest) IsPresign() bool {
	return request.WorkType == EcdsaPresign || request.WorkType == EddsaPresign
}

func (request *WorkRequest) IsSigning() bool {
	return request.WorkType == EcdsaSigning || request.WorkType == EddsaSigning
}
