package server

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"time"

	eTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/sisu-network/dheart/client"
	"github.com/sisu-network/dheart/common"
	"github.com/sisu-network/dheart/store"
	"github.com/sisu-network/dheart/types"
	"github.com/sisu-network/dheart/utils"
)

const (
	encodedAESKey = "Jxq9PhUzvP4RZFBQivXGfA"
)

// This is a mock API to use for single localhost node. It does not have TSS signing round and
// generates a private key instead.
type SingleNodeApi struct {
	keyMap   map[string]interface{}
	store    store.Store
	ethKeys  map[string]*ecdsa.PrivateKey
	chainIds map[string]*big.Int
	c        client.Client
}

func NewSingleNodeApi(c client.Client, store store.Store) *SingleNodeApi {
	return &SingleNodeApi{
		keyMap:   make(map[string]interface{}),
		ethKeys:  make(map[string]*ecdsa.PrivateKey),
		c:        c,
		store:    store,
		chainIds: make(map[string]*big.Int),
	}
}

// Initializes private keys used for dheart
func (api *SingleNodeApi) Init() {
	// Initialized keygens
	for _, chain := range common.SUPPORTED_CHAINS {
		bz, err := api.store.GetEncrypted([]byte(api.getKeygenKey(chain)))
		if err != nil {
			continue
		}

		if utils.IsEcDSA(chain) {
			privKey, err := crypto.ToECDSA(bz)
			if err != nil {
				panic(err)
			}

			api.ethKeys[chain] = privKey
		}
	}

	// TODO: Add more chain ids here.
	api.chainIds["eth"] = big.NewInt(1)
}

// Empty function for checking health only.
func (api *SingleNodeApi) CheckHealth() {
}

func (api *SingleNodeApi) Version() string {
	return "1"
}

func (api *SingleNodeApi) KeyGen(keygenId string, chain string, tPubKeys []types.PubKeyWrapper) error {
	utils.LogInfo("keygen: chain = ", chain)
	var err error
	switch chain {
	case "eth":
		err = api.keyGenEth(chain)
	default:
		return fmt.Errorf("Unknown chain: %s", chain)
	}

	utils.LogInfo("err = ", err)
	pubKey := api.ethKeys[chain].Public()
	publicKeyECDSA, _ := pubKey.(*ecdsa.PublicKey)
	publicKeyBytes := crypto.CompressPubkey(publicKeyECDSA)
	address := crypto.PubkeyToAddress(*publicKeyECDSA).Hex()

	if err == nil {
		// Add some delay to mock TSS gen delay before sending back to Sisu server
		go func() {
			time.Sleep(time.Second * 3)
			utils.LogInfo("Sending keygen to Sisu")

			if err := api.c.BroadcastKeygenResult(chain, publicKeyBytes, address); err != nil {
				utils.LogError("Error while broadcasting KeygenResult", err)
			}
		}()
	} else {
		utils.LogError(err)
	}

	return err
}

func (api *SingleNodeApi) getKeygenKey(chain string) []byte {
	return []byte(fmt.Sprintf("keygen_%s", chain))
}

// Key generation for ETH based chains
func (api *SingleNodeApi) keyGenEth(chain string) error {
	utils.LogInfo("Keygen for chain", chain)
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		return err
	}

	api.ethKeys[chain] = privateKey

	encoded := crypto.FromECDSA(privateKey)
	return api.store.PutEncrypted(api.getKeygenKey(chain), encoded)
}

func (api *SingleNodeApi) keySignEth(chain string, serialized []byte) ([]byte, []byte, error) {
	if api.ethKeys[chain] == nil {
		return nil, nil, fmt.Errorf("There is no private key for this chain")
	}

	privateKey := api.ethKeys[chain]
	tx := &eTypes.Transaction{}
	err := tx.UnmarshalBinary(serialized)
	if err != nil {
		utils.LogError("Cannot unmarshall ETH tx.")
		return nil, nil, err
	}

	signer := eTypes.NewEIP2930Signer(api.chainIds[chain])
	h := signer.Hash(tx)
	sig, err := crypto.Sign(h[:], privateKey)

	return sig, crypto.FromECDSAPub(&privateKey.PublicKey), err
}

// Signing any transaction
func (api *SingleNodeApi) KeySign(req *types.KeysignRequest) error {
	var err error
	var pubkey, signature []byte

	utils.LogDebug("Signing transaction....")
	switch req.OutChain {
	case "eth":
		signature, pubkey, err = api.keySignEth(req.OutChain, req.OutBytes)
	default:
		return fmt.Errorf("Unknown chain: %s", req.OutChain)
	}

	if err == nil {
		// Add some delay to mock TSS gen delay before sending back to Sisu server
		go func() {
			time.Sleep(time.Second * 3)
			utils.LogInfo("Sending Keysign to Sisu")

			api.c.BroadcastKeySignResult(&types.KeysignResult{
				Success:        true,
				OutChain:       req.OutChain,
				OutBlockHeight: req.OutBlockHeight,
				OutHash:        req.OutHash,
				PubKey:         pubkey,
				OutBytes:       req.OutBytes,
				Signature:      signature,
			})
		}()
	} else {
		utils.LogError("Cannot do key gen. Err =", err)
		api.c.BroadcastKeySignResult(&types.KeysignResult{
			Success:        false,
			ErrMesage:      err.Error(),
			OutChain:       req.OutChain,
			OutBlockHeight: req.OutBlockHeight,
			OutHash:        req.OutHash,
			OutBytes:       req.OutBytes,
			Signature:      signature,
		})
	}

	return err
}

func (api *SingleNodeApi) SetPrivKey(encodedKey string, keyType string) error {
	return nil
}
