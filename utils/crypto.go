package utils

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"

	"github.com/sisu-network/tuktuk/common"
)

func EncodeEcDSA(privateKey *ecdsa.PrivateKey) []byte {
	x509Encoded, _ := x509.MarshalECPrivateKey(privateKey)
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: x509Encoded})
}

func DecodeEcDSA(encoded []byte) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(encoded))
	x509Encoded := block.Bytes
	privateKey, err := x509.ParseECPrivateKey(x509Encoded)

	return privateKey, err
}

func IsEcDSA(chain string) bool {
	return chain == common.CHAIN_ETH || chain == common.CHAIN_ETH_MAINNET
}
