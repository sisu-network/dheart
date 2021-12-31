package types

import "github.com/sisu-network/tss-lib/tss"

type KeysignRequest struct {
	Id          string
	InChain     string
	OutChain    string
	OutHash     string
	BytesToSign []byte
}

type KeysignResult struct {
	Request   *KeysignRequest
	Success   bool
	ErrMesage string

	Signature []byte

	Culprits []*tss.PartyID
}
