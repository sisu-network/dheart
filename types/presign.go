package types

import "github.com/sisu-network/tss-lib/tss"

type PresignResult struct {
	Outcome OutcomeType

	PubKeyBytes []byte
	Address     string
	Culprits    []*tss.PartyID
}
