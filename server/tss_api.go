package server

import (
	"fmt"

	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	ctypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/sisu-network/lib/log"

	"github.com/sisu-network/dheart/core"
	"github.com/sisu-network/dheart/types"
)

type TssApi struct {
	heart *core.Heart
}

func NewTssApi(heart *core.Heart) *TssApi {
	return &TssApi{
		heart: heart,
	}
}

func (api *TssApi) Init() {
	api.heart.Start()
}

func (api *TssApi) Version() string {
	return "1"
}

func (api *TssApi) Ping(source string) {
	// Do nothing.
}

func (api *TssApi) KeyGen(keygenId string, chain string, keyWrappers []types.PubKeyWrapper) error {
	if len(keyWrappers) == 0 {
		return fmt.Errorf("invalid keys array cannot be empty")
	}

	log.Info("There is a keygen request ", keygenId, chain)

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

	return api.heart.Keygen(keygenId, chain, pubKeys)
}

// SetSisuReady sets the Sisu's readiness state. This informs dheart that Sisu is ready and dheart
// can be fully functional.
func (api *TssApi) SetSisuReady(isReady bool) {
	api.heart.SetSisuReady(isReady)
}

func (api *TssApi) SetPrivKey(encodedKey string, keyType string) error {
	log.Info("Setting private key, key type =", keyType)
	err := api.heart.SetPrivKey(encodedKey, keyType)
	log.Verbose("Done setting private key, err = ", err)

	return err
}

func (api *TssApi) getPubkeysFromWrapper(keyWrappers []types.PubKeyWrapper) ([]ctypes.PubKey, error) {
	pubKeys := make([]ctypes.PubKey, len(keyWrappers))

	for i, wrapper := range keyWrappers {
		keyType := wrapper.KeyType
		switch keyType {
		case "ed25519":
			pubKeys[i] = &ed25519.PubKey{Key: wrapper.Key}
		case "secp256k1":
			pubKeys[i] = &secp256k1.PubKey{Key: wrapper.Key}
		default:
			return make([]ctypes.PubKey, 0), fmt.Errorf("unknown key type %s", keyType)
		}
	}

	return pubKeys, nil
}

func (api *TssApi) KeySign(req *types.KeysignRequest, keyWrappers []types.PubKeyWrapper) error {
	log.Info("There is keysign request with message length ", len(req.KeysignMessages))

	pubKeys, err := api.getPubkeysFromWrapper(keyWrappers)
	if err != nil {
		log.Error("Failed to get pubkey, err =", err)
		return err
	}

	err = api.heart.Keysign(req, pubKeys)
	if err != nil {
		log.Error("Cannot do keysign, err =", err)
	}

	return err
}

func (api *TssApi) Reshare(req *types.ReshareRequest) error {
	return nil
}

func (api *TssApi) BlockEnd(blockHeight int64) error {
	return api.heart.BlockEnd(blockHeight)
}
