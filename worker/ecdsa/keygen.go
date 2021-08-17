package ecdsa

// Implements Worker interface
type KeygenWorker struct {
}

func NewKeygenWorker() *KeygenWorker {
	return &KeygenWorker{}
}

func (kgw *KeygenWorker) Start() error {
	return nil
}
