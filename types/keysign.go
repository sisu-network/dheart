package types

type KeysignRequest struct {
	OutChain       string
	OutHash        string
	OutBlockHeight int64
	OutBytes       []byte
}

type KeysignResult struct {
	OutChain       string
	OutHash        string
	OutBlockHeight int64

	Signature []byte
}
