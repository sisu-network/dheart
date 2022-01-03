package types

import "github.com/sisu-network/tss-lib/tss"

type KeysignRequest struct {
	KeyType         string
	KeysignMessages []*KeysignMessage
}

type KeysignMessage struct {
	Id          string
	InChain     string
	OutChain    string
	OutHash     string
	BytesToSign []byte
}

type KeysignResult struct {
	Request *KeysignRequest

	Success   bool
	ErrMesage string

	Signatures [][]byte

	Culprits []*tss.PartyID
}
