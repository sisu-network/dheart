package types

import (
	"errors"

	"github.com/sisu-network/tss-lib/ecdsa/keygen"
	"github.com/sisu-network/tss-lib/ecdsa/presign"
	"github.com/sisu-network/tss-lib/tss"
)

type WorkRequest struct {
	WorkType WorkType
	PIDs     tss.SortedPartyIDs
	WorkId   string

	// Used only for keygen, presign & signing
	KeygenInput *keygen.LocalPreParams
	Threshold   int

	// Used for presign
	PresignInput *keygen.LocalPartySaveData

	// Used for signing
	SigningInput []*presign.LocalPresignData
	Message      string
}

func NewKeygenRequest(workId string, PIDs tss.SortedPartyIDs, keygenInput keygen.LocalPreParams, threshold int) *WorkRequest {
	request := baseRequest(ECDSA_KEYGEN, workId, PIDs)
	request.KeygenInput = &keygenInput
	request.Threshold = threshold

	return request
}

func NewPresignRequest(workId string, PIDs tss.SortedPartyIDs, presignInput keygen.LocalPartySaveData) *WorkRequest {
	request := baseRequest(ECDSA_PRESIGN, workId, PIDs)
	request.PresignInput = &presignInput

	return request
}

func NewSigningRequets(workId string, PIDs tss.SortedPartyIDs, signingInput []*presign.LocalPresignData, message string) *WorkRequest {
	request := baseRequest(ECDSA_SIGNING, workId, PIDs)
	request.SigningInput = signingInput
	request.Message = message

	return request
}

func baseRequest(workType WorkType, workdId string, pIDs tss.SortedPartyIDs) *WorkRequest {
	return &WorkRequest{
		WorkType: workType,
		WorkId:   workdId,
		PIDs:     pIDs,
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
	case ECDSA_SIGNING:
		if request.SigningInput == nil || len(request.SigningInput) == 0 {
			return errors.New("Signing input could not be nil or empty for signing task")
		}
	case EDDSA_KEYGEN:
	case EDDSA_PRESIGN:
	case EDDSA_SIGNING:
	default:
		return errors.New("Invalid request type")
	}
	return nil
}
