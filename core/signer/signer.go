package signer

import (
	tcrypto "github.com/tendermint/tendermint/crypto"
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
