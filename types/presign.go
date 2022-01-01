package types

import "github.com/sisu-network/tss-lib/tss"

type PresignResult struct {
	// Chain       string
	Success     bool
	PubKeyBytes []byte
	Address     string
	Culprits    []*tss.PartyID
}
