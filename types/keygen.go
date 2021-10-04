package types

import (
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/sisu-network/tss-lib/tss"
)

type KeygenResult struct {
	Chain       string
	Success     bool
	PubKeyBytes []byte
	Address     string
	Culprits    []*tss.PartyID
}

// Pubkey wrapper around cosmos pubkey type to avoid unmarshaling exception in rpc server.
type PubKeyWrapper struct {
	KeyType   string
	Key       []byte
	Ed25519   *ed25519.PubKey
	Secp256k1 *secp256k1.PubKey
}
