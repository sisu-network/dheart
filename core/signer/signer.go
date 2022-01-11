package signer

import (
	ctypes "github.com/cosmos/cosmos-sdk/crypto/types"
)

type Signer interface {
	Sign(msg []byte) ([]byte, error)
	PubKey() ctypes.PubKey
}

type DefaultSigner struct {
	privateKey ctypes.PrivKey
}

func NewDefaultSigner(privateKey ctypes.PrivKey) *DefaultSigner {
	return &DefaultSigner{
		privateKey: privateKey,
	}
}

func (signer *DefaultSigner) Sign(msg []byte) ([]byte, error) {
	return signer.privateKey.Sign(msg)
}

func (signer *DefaultSigner) PubKey() ctypes.PubKey {
	return signer.privateKey.PubKey()
}
