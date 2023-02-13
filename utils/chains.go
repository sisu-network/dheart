package utils

import "github.com/echovl/cardano-go"

func GetAddressFromCardanoPubkey(pubkey []byte) (cardano.Address, error) {
	keyHash, err := cardano.Blake224Hash(pubkey)
	if err != nil {
		return cardano.Address{}, err
	}

	payment := cardano.StakeCredential{Type: cardano.KeyCredential, KeyHash: keyHash}
	enterpriseAddr, err := cardano.NewEnterpriseAddress(0, payment)
	if err != nil {
		return cardano.Address{}, err
	}

	return enterpriseAddr, nil
}
