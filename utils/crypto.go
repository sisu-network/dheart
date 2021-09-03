package utils

import (
	"crypto/aes"
	"crypto/cipher"

	"github.com/sisu-network/dheart/common"
)

func IsEcDSA(chain string) bool {
	return chain == common.CHAIN_ETH || chain == common.CHAIN_ETH_MAINNET
}

func AESDecrypt(encrypted []byte, key []byte) ([]byte, error) {
	c, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(c)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	nonce, encrypted := encrypted[:nonceSize], encrypted[nonceSize:]
	decrypted, err := gcm.Open(nil, nonce, encrypted, nil)

	return decrypted, err
}
