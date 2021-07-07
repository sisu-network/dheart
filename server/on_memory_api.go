package server

import (
	"crypto/ecdsa"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/sisu-network/tuktuk/core"
	"github.com/sisu-network/tuktuk/utils"
)

type OnMemoryApi struct {
	tutuk *core.TutTuk

	keyMap map[string]interface{}
	ethKey *ecdsa.PrivateKey
}

func NewOnMemoryApi(tuktuk *core.TutTuk) *OnMemoryApi {
	return &OnMemoryApi{
		tutuk:  tuktuk,
		keyMap: make(map[string]interface{}),
	}
}

func (api *OnMemoryApi) Version() string {
	return "1"
}

func (api *OnMemoryApi) KeyGen(chain string) error {
	utils.LogInfo("chain = ", chain)
	switch chain {
	case "eth":
		return api.keyGenEth(chain)
	}
	return nil
}

func (api *OnMemoryApi) keyGenEth(chain string) error {
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		return err
	}

	api.ethKey = privateKey
	return nil
}
