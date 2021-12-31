package types

import (
	"github.com/sisu-network/tss-lib/tss"
)

type KeygenResult struct {
	KeyType     string
	KeygenIndex int
	Success     bool
	PubKeyBytes []byte
	Address     string
	Culprits    []*tss.PartyID
}

// Pubkey wrapper around cosmos pubkey type to avoid unmarshaling exception in rpc server.
type PubKeyWrapper struct {
	KeyType string
	Key     []byte
}
