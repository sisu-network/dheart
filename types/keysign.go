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
	Bytes       []byte
	BytesToSign []byte
}

type KeysignResult struct {
	Outcome    OutcomeType
	Request    *KeysignRequest
	ErrMesage  string
	Signatures [][]byte
	Culprits   []*tss.PartyID
}
