package types

type KeygenResult struct {
	Chain       string
	Success     bool
	PubKeyBytes []byte
	Address     string
}
