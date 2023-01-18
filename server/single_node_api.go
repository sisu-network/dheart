package server

import (
	"crypto/ecdsa"
	"fmt"
	"time"

	"github.com/decred/dcrd/dcrec/edwards/v2"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/sisu-network/dheart/client"
	"github.com/sisu-network/dheart/types"
	"github.com/sisu-network/dheart/utils"
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
	c      client.Client

	ecPrivate *ecdsa.PrivateKey
	edPrivate *edwards.PrivateKey
}

func NewSingleNodeApi(c client.Client) *SingleNodeApi {
	return &SingleNodeApi{
		keyMap: make(map[string]interface{}),
		c:      c,
	}
}

// Init implements Api interface.
func (api *SingleNodeApi) Init() {
	// Do nothing
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

	// Add some delay to mock TSS gen delay before sending back to Sisu server
	go func() {
		time.Sleep(time.Second * 3)
		log.Info("Sending keygen to Sisu")

		var result types.KeygenResult
		switch keyType {
		case libchain.KEY_TYPE_ECDSA:
			var err error
			api.ecPrivate, err = crypto.GenerateKey()
			if err != nil {
				panic(err)
			}

			pubKey := api.ecPrivate.Public()
			publicKeyECDSA, _ := pubKey.(*ecdsa.PublicKey)
			publicKeyBytes := crypto.FromECDSAPub(publicKeyECDSA)
			address := crypto.PubkeyToAddress(*publicKeyECDSA).Hex()

			result = types.KeygenResult{
				KeyType:     keyType,
				Outcome:     types.OutcomeSuccess,
				Address:     address,
				PubKeyBytes: publicKeyBytes,
			}
		case libchain.KEY_TYPE_EDDSA:
			api.edPrivate, _ = edwards.GeneratePrivateKey()
			pubKeyBytes := api.edPrivate.PubKey().Serialize()

			result = types.KeygenResult{
				KeyType:     keyType,
				Outcome:     types.OutcomeSuccess,
				Address:     utils.GetAddressFromCardanoPubkey(pubKeyBytes).String(),
				PubKeyBytes: pubKeyBytes,
			}
		}

		if err := api.c.PostKeygenResult(&result); err != nil {
			log.Error("Error while broadcasting KeygenResult", err)
		}
	}()

	return nil
}

func (api *SingleNodeApi) getKeygenKey(chain string) []byte {
	return []byte(fmt.Sprintf("keygen_%s", chain))
}

func (api *SingleNodeApi) keySignEth(chain string, bytesToSign []byte) ([]byte, error) {
	privateKey := api.ecPrivate
	sig, err := crypto.Sign(bytesToSign, privateKey)

	return sig, err
}

func (api *SingleNodeApi) keySignEddsa(chain string, bytesToSign []byte) ([]byte, error) {
	sig, err := api.edPrivate.Sign(bytesToSign)
	return sig.Serialize(), err
}

// Signing any transaction
func (api *SingleNodeApi) KeySign(req *types.KeysignRequest, tPubKeys []types.PubKeyWrapper) error {
	var err error

	signatures := make([][]byte, len(req.KeysignMessages))

	for i, msg := range req.KeysignMessages {
		var signature []byte
		var err error

		switch req.KeyType {
		case libchain.KEY_TYPE_ECDSA:
			signature, err = api.keySignEth(msg.OutChain, msg.BytesToSign)

		case libchain.KEY_TYPE_EDDSA:
			signature, err = api.keySignEddsa(msg.OutChain, msg.BytesToSign)
		default:
			err = fmt.Errorf("Unknown chain: %s for message at index %d", msg.OutChain, i)
		}

		if err != nil {
			return err
		}

		signatures[i] = signature
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

func (api *SingleNodeApi) SetPrivKey(encodedKey string, keyType string) error {
	return nil
}

func (api *SingleNodeApi) BlockEnd(blockHeight int64) error {
	// Do nothing.
	return nil
}
