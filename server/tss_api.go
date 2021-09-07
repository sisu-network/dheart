package server

import (
	"fmt"

	"github.com/ethereum/go-ethereum/core/types"
	common "github.com/sisu-network/dheart/common"
	"github.com/sisu-network/dheart/core"
)

type TssApi struct {
	isSetup bool
	heart   *core.Heart
}

func NewTssApi(heart *core.Heart) *TssApi {
	return &TssApi{
		heart: heart,
	}
}

func (api *TssApi) Version() string {
	return "1"
}

func (api *TssApi) DoKeygen() {
}

func (api *TssApi) SignEthTx(chainSymbol string, tx *types.Transaction) {
}

// This function should only call one during the entire process cycle. If the caller wants to
// call the second time, this tss process should be restarted.
func (api *TssApi) Setup(configs []common.ChainConfig) error {
	if api.isSetup {
		return fmt.Errorf("Setup function should only be called once.")
	}
	api.isSetup = true

	return nil
}

func (api *TssApi) SetPrivKey(encodedKey string, keyType string) error {
	return api.heart.SetPrivKey(encodedKey, keyType)
}
