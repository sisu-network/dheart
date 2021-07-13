package server

import (
	"crypto/ecdsa"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/sisu-network/tuktuk/core"
	"github.com/sisu-network/tuktuk/utils"
)

// This is a mock API to use for single localhost node. It does not have TSS signing round and
// generates a private key instead.
type SingleNodeApi struct {
	tutuk *core.TutTuk

	keyMap map[string]interface{}
	ethKey *ecdsa.PrivateKey
}

func NewSingleNodeApi(tuktuk *core.TutTuk) *SingleNodeApi {
	return &SingleNodeApi{
		tutuk:  tuktuk,
		keyMap: make(map[string]interface{}),
	}
}

func (api *SingleNodeApi) Version() string {
	return "1"
}

func (api *SingleNodeApi) KeyGen(chain string) error {
	utils.LogInfo("chain = ", chain)
	switch chain {
	case "eth":
		return api.keyGenEth(chain)
	}

	return nil
}

func (api *SingleNodeApi) keyGenEth(chain string) error {
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		return err
	}

	api.ethKey = privateKey
	return nil
}
