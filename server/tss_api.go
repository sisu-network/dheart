package server

import (
	"fmt"

	"github.com/ethereum/go-ethereum/core/types"
	common "github.com/sisu-network/dheart/common"
	"github.com/sisu-network/dheart/core"
)

type TssApi struct {
	isSetup bool
	dheart  *core.Dheart
}

func NewTssApi(dheart *core.Dheart) *TssApi {
	return &TssApi{
		dheart: dheart,
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

func (api *TssApi) Halt() {

}
