package types

import "github.com/sisu-network/tss-lib/tss"

type KeysignRequest struct {
	Id             string
	OutChain       string
	OutHash        string
	OutBlockHeight int64
	OutBytes       []byte
}

type KeysignResult struct {
	Success   bool
	ErrMesage string

	Id             string
	OutChain       string
	OutHash        string
	OutBlockHeight int64

	OutBytes  []byte
	Signature []byte

	Culprits []*tss.PartyID
}
