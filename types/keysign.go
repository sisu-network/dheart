package types

type KeysignRequest struct {
	Chain         string
	OutTxHash     string
	InBlockHeight int64
	OutBytes      []byte
}

type KeysignResult struct {
	Chain         string
	OutTxHash     string
	InBlockHeight int64

	Signature []byte
}
