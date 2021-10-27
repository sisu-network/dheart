package types

import (
	"errors"

	"github.com/sisu-network/dheart/utils"
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
	KeygenInput *keygen.LocalPreParams
	Threshold   int

	// Used for presign
	PresignInput *keygen.LocalPartySaveData

	// Used for signing
	Message string
}

func NewKeygenRequest(chain, workId string, n int, PIDs tss.SortedPartyIDs, keygenInput keygen.LocalPreParams, threshold int) *WorkRequest {
	request := baseRequest(EcdsaKeygen, chain, workId, n, PIDs)
	request.KeygenInput = &keygenInput
	request.Threshold = threshold

	return request
}

func NewPresignRequest(chain, workId string, n int, PIDs tss.SortedPartyIDs, presignInput keygen.LocalPartySaveData, forcedPresign bool) *WorkRequest {
	request := baseRequest(EcdsaPresign, chain, workId, n, PIDs)
	request.PresignInput = &presignInput
	request.ForcedPresign = forcedPresign

	return request
}

func NewSigningRequets(chain, workId string, n int, PIDs tss.SortedPartyIDs, message string) *WorkRequest {
	request := baseRequest(EcdsaSigning, chain, workId, n, PIDs)
	request.Message = message

	return request
}

func baseRequest(workType WorkType, chain, workdId string, n int, pIDs tss.SortedPartyIDs) *WorkRequest {
	return &WorkRequest{
		Chain:      chain,
		AllParties: pIDs,
		WorkType:   workType,
		WorkId:     workdId,
		N:          n,
	}
}

func (request *WorkRequest) Validate() error {
	switch request.WorkType {
	case EcdsaKeygen:
		if request.KeygenInput == nil {
			return errors.New("Keygen input could not be nil for keygen task")
		}
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

	utils.LogCritical("Unknown work type", request.WorkType)

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
