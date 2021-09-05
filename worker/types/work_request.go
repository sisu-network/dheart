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
	N             int // The number of avaiable participants required to do this task.
	ForcedPresign bool

	// Used only for keygen, presign & signing
	KeygenInput *keygen.LocalPreParams
	Threshold   int

	// Used for presign
	PresignInput *keygen.LocalPartySaveData

	// Used for signing
	Message string
}

func NewKeygenRequest(workId string, n int, PIDs tss.SortedPartyIDs, keygenInput keygen.LocalPreParams, threshold int) *WorkRequest {
	request := baseRequest(ECDSA_KEYGEN, workId, n, PIDs)
	request.KeygenInput = &keygenInput
	request.Threshold = threshold

	return request
}

func NewPresignRequest(workId string, n int, PIDs tss.SortedPartyIDs, presignInput keygen.LocalPartySaveData, forcedPresign bool) *WorkRequest {
	request := baseRequest(ECDSA_PRESIGN, workId, n, PIDs)
	request.PresignInput = &presignInput
	request.ForcedPresign = forcedPresign

	return request
}

func NewSigningRequets(workId string, n int, PIDs tss.SortedPartyIDs, message string) *WorkRequest {
	request := baseRequest(ECDSA_SIGNING, workId, n, PIDs)
	request.Message = message

	return request
}

func baseRequest(workType WorkType, workdId string, n int, pIDs tss.SortedPartyIDs) *WorkRequest {
	return &WorkRequest{
		AllParties: pIDs,
		WorkType:   workType,
		WorkId:     workdId,
		N:          n,
	}
}

func (request *WorkRequest) Validate() error {
	switch request.WorkType {
	case ECDSA_KEYGEN:
		if request.KeygenInput == nil {
			return errors.New("Keygen input could not be nil for keygen task")
		}
	case ECDSA_PRESIGN:
		if request.PresignInput == nil {
			return errors.New("Presign input could not be nil for presign task")
		}
	case EDDSA_KEYGEN:
	case EDDSA_PRESIGN:
	case EDDSA_SIGNING:
	default:
		return errors.New("Invalid request type")
	}
	return nil
}

func (request *WorkRequest) GetPriority() int {
	// Keygen
	if request.WorkType == ECDSA_KEYGEN || request.WorkType == EDDSA_KEYGEN {
		return 100
	}

	if request.WorkType == ECDSA_PRESIGN || request.WorkType == EDDSA_PRESIGN {
		if request.ForcedPresign {
			// Force presign
			return 90
		}

		// Presign
		return 70
	}

	// Signing
	if request.WorkType == ECDSA_SIGNING || request.WorkType == EDDSA_SIGNING {
		return 60
	}

	utils.LogCritical("Unknown work type", request.WorkType)

	return -1
}
