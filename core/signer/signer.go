package signer

import (
	tcrypto "github.com/cosmos/cosmos-sdk/crypto/types"
)

type Signer interface {
	Sign(msg []byte) ([]byte, error)
	PubKey() tcrypto.PubKey
}

type DefaultSigner struct {
	privateKey tcrypto.PrivKey
}

func NewDefaultSigner(privateKey tcrypto.PrivKey) *DefaultSigner {
	return &DefaultSigner{
		privateKey: privateKey,
	}
}

func (signer *DefaultSigner) Sign(msg []byte) ([]byte, error) {
	return signer.privateKey.Sign(msg)
}

func (signer *DefaultSigner) PubKey() tcrypto.PubKey {
	return signer.privateKey.PubKey()
}
