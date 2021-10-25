package server

import (
	"fmt"

	"github.com/sisu-network/cosmos-sdk/crypto/keys/ed25519"
	"github.com/sisu-network/cosmos-sdk/crypto/keys/secp256k1"
	ctypes "github.com/sisu-network/cosmos-sdk/crypto/types"

	etypes "github.com/ethereum/go-ethereum/core/types"
	common "github.com/sisu-network/dheart/common"
	"github.com/sisu-network/dheart/core"
	"github.com/sisu-network/dheart/types"
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

func (api *TssApi) Init() {

}

func (api *TssApi) Version() string {
	return "1"
}

func (api *TssApi) KeyGen(keygenId string, chain string, keyWrappers []types.PubKeyWrapper) error {
	if len(keyWrappers) == 0 {
		return fmt.Errorf("invalid keys array cannot be empty")
	}

	pubKeys := make([]ctypes.PubKey, len(keyWrappers))

	for i, wrapper := range keyWrappers {
		keyType := wrapper.KeyType
		switch keyType {
		case "ed25519":
			pubKeys[i] = &ed25519.PubKey{Key: wrapper.Key}
		case "secp256k1":
			pubKeys[i] = &secp256k1.PubKey{Key: wrapper.Key}
		}
	}

	go api.heart.Keygen(keygenId, chain, pubKeys)

	return nil
}

func (api *TssApi) SignEthTx(chainSymbol string, tx *etypes.Transaction) {
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

func (api *TssApi) KeySign(req *types.KeysignRequest) error {
	return nil
}
