package server

import (
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/sisu-network/dheart/client"
	"github.com/sisu-network/dheart/store"
	"github.com/sisu-network/dheart/types"
	"github.com/sisu-network/lib/log"

	libchain "github.com/sisu-network/lib/chain"
)

const (
	encodedAESKey = "Jxq9PhUzvP4RZFBQivXGfA"
)

// This is a mock API to use for single localhost node. It does not have TSS signing round and
// generates a private key instead.
type SingleNodeApi struct {
	keyMap map[string]interface{}
	store  store.Store
	ecPriv *ecdsa.PrivateKey
	c      client.Client
}

func NewSingleNodeApi(c client.Client, store store.Store) *SingleNodeApi {
	return &SingleNodeApi{
		keyMap: make(map[string]interface{}),
		c:      c,
		store:  store,
	}
}

// Initializes private keys used for dheart
func (api *SingleNodeApi) Init() {
	// Initialized keygens
	var err error
	api.ecPriv, err = crypto.GenerateKey()
	if err != nil {
		panic(err)
	}
}

// SetSisuReady implements Api interface.
func (api *SingleNodeApi) SetSisuReady(isReady bool) {
	// Do nothing.
}

// Empty function for checking health only.
func (api *SingleNodeApi) Ping(source string) {
}

func (api *SingleNodeApi) Version() string {
	return "1"
}

func (api *SingleNodeApi) KeyGen(keygenId string, keyType string, tPubKeys []types.PubKeyWrapper) error {
	log.Info("keygen: keyType = ", keyType)
	var err error

	if keyType != libchain.KEY_TYPE_ECDSA {
		return errors.New("In")
	}

	log.Info("err = ", err)
	pubKey := api.ecPriv.Public()
	publicKeyECDSA, _ := pubKey.(*ecdsa.PublicKey)
	publicKeyBytes := crypto.FromECDSAPub(publicKeyECDSA)
	address := crypto.PubkeyToAddress(*publicKeyECDSA).Hex()

	if err == nil {
		// Add some delay to mock TSS gen delay before sending back to Sisu server
		go func() {
			time.Sleep(time.Second * 3)
			log.Info("Sending keygen to Sisu")

			result := types.KeygenResult{
				KeyType:     keyType,
				Outcome:     types.OutcomeSuccess,
				PubKeyBytes: publicKeyBytes,
				Address:     address,
			}
			if err := api.c.PostKeygenResult(&result); err != nil {
				log.Error("Error while broadcasting KeygenResult", err)
			}
		}()
	} else {
		log.Error(err)
	}

	return err
}

func (api *SingleNodeApi) getKeygenKey(chain string) []byte {
	return []byte(fmt.Sprintf("keygen_%s", chain))
}

func (api *SingleNodeApi) keySignEth(chain string, bytesToSign []byte) ([]byte, error) {
	privateKey := api.ecPriv
	sig, err := crypto.Sign(bytesToSign, privateKey)

	return sig, err
}

// Signing any transaction
func (api *SingleNodeApi) KeySign(req *types.KeysignRequest, tPubKeys []types.PubKeyWrapper) error {
	var err error

	signatures := make([][]byte, len(req.KeysignMessages))

	for i, msg := range req.KeysignMessages {
		if libchain.IsETHBasedChain(msg.OutChain) {
			signature, err := api.keySignEth(msg.OutChain, msg.BytesToSign)
			if err == nil {
				signatures[i] = signature
			}
		} else {
			return fmt.Errorf("Unknown chain: %s for message at index %d", msg.OutChain, i)
		}
	}

	if err == nil {
		// Add some delay to mock TSS gen delay before sending back to Sisu server
		go func() {
			time.Sleep(time.Second * 3)
			log.Info("Sending Keysign to Sisu")

			result := &types.KeysignResult{
				Request:    req,
				Outcome:    types.OutcomeSuccess,
				Signatures: signatures,
			}

			api.c.PostKeysignResult(result)
		}()
	} else {
		log.Error("Cannot do key gen. Err =", err)
		api.c.PostKeysignResult(&types.KeysignResult{
			Request:   req,
			Outcome:   types.OutcomeFailure,
			ErrMesage: err.Error(),
		})
	}

	return err
}

func (api *SingleNodeApi) Reshare(req *types.ReshareRequest) error {
	s, _ := json.Marshal(req)
	log.Debug("Reshare request from sisu = ", string(s))
	return api.c.PostReshareResult(&types.ReshareResult{
		Outcome:                    types.OutcomeSuccess,
		NewValidatorSetPubKeyBytes: req.NewValidatorSetPubKeyBytes,
	})
}

func (api *SingleNodeApi) SetPrivKey(encodedKey string, keyType string) error {
	return nil
}

func (api *SingleNodeApi) BlockEnd(blockHeight int64) error {
	// Do nothing.
	return nil
}
