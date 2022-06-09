package types

import (
	"crypto/ecdsa"

	"github.com/decred/dcrd/dcrec/edwards/v2"
	"github.com/sisu-network/tss-lib/tss"
)

type KeygenResult struct {
	KeyType     string
	KeygenIndex int

	Outcome  OutcomeType
	Address  string
	Culprits []*tss.PartyID

	EcdsaPubkey *ecdsa.PublicKey
	EddsaPubkey *edwards.PublicKey
}

// Pubkey wrapper around cosmos pubkey type to avoid unmarshaling exception in rpc server.
type PubKeyWrapper struct {
	KeyType string
	Key     []byte
}
