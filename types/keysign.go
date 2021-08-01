package types

type KeysignRequest struct {
	OutChain       string
	OutHash        string
	OutBlockHeight int64
	OutBytes       []byte
}

type KeysignResult struct {
	Success   bool
	ErrMesage string

	OutChain       string
	OutHash        string
	OutBlockHeight int64

	Signature []byte
}
