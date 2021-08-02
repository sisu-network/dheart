package server

import (
	"crypto/ecdsa"
	"encoding/base64"
	"fmt"
	"math/big"
	"os"
	"time"

	eTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/sisu-network/tuktuk/client"
	"github.com/sisu-network/tuktuk/common"
	"github.com/sisu-network/tuktuk/core"
	"github.com/sisu-network/tuktuk/store"
	"github.com/sisu-network/tuktuk/types"
	"github.com/sisu-network/tuktuk/utils"
)

const (
	encodedAESKey = "Jxq9PhUzvP4RZFBQivXGfA"
)

// This is a mock API to use for single localhost node. It does not have TSS signing round and
// generates a private key instead.
type SingleNodeApi struct {
	tutuk    *core.TutTuk
	keyMap   map[string]interface{}
	store    *store.Store
	ethKeys  map[string]*ecdsa.PrivateKey
	chainIds map[string]*big.Int
	c        *client.Client
}

func NewSingleNodeApi(tuktuk *core.TutTuk, c *client.Client) *SingleNodeApi {
	return &SingleNodeApi{
		tutuk:   tuktuk,
		keyMap:  make(map[string]interface{}),
		ethKeys: make(map[string]*ecdsa.PrivateKey),
		c:       c,
	}
}

// Initializes private keys used for Tuktuk
func (api *SingleNodeApi) Init() {
	// Store
	aesKey, err := base64.RawStdEncoding.DecodeString(encodedAESKey)
	if err != nil {
		panic(err)
	}

	path := os.Getenv("HOME_DIR")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		err := os.Mkdir(path, os.ModePerm)
		if err != nil {
			panic(err)
		}
	}

	store, err := store.NewStore(path+"/apidb", aesKey)
	if err != nil {
		panic(err)
	}
	api.store = store

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
}

// Empty function for checking health only.
func (api *SingleNodeApi) CheckHealth() {
}

func (api *SingleNodeApi) Version() string {
	return "1"
}

func (api *SingleNodeApi) KeyGen(chain string) error {
	utils.LogInfo("chain = ", chain)
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

	if err == nil {
		// Add some delay to mock TSS gen delay before sending back to Sisu server
		go func() {
			time.Sleep(time.Second * 3)
			utils.LogInfo("Sending keygen to Sisu")

			api.c.BroadcastKeygenResult(chain, publicKeyBytes)
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

func (api *SingleNodeApi) keySignEth(chain string, serialized []byte) ([]byte, error) {
	if api.ethKeys[chain] == nil {
		return nil, fmt.Errorf("There is no private key for this chain")
	}

	privateKey := api.ethKeys[chain]
	tx := &eTypes.Transaction{}
	err := tx.UnmarshalBinary(serialized)
	if err != nil {
		utils.LogError("Cannot unmarshall ETH tx.")
		return nil, err
	}

	signedTx, err := eTypes.SignTx(tx, eTypes.NewEIP155Signer(api.chainIds[chain]), privateKey)
	if err != nil {
		utils.LogError("cannot sign eth tx. err = ", err)
	}

	utils.LogInfo("Signing completed")

	serializedSigned, err := signedTx.MarshalBinary()

	return serializedSigned, err
}

// Signing any transaction
func (api *SingleNodeApi) KeySign(req *types.KeysignRequest) error {
	var err error
	var signature []byte

	utils.LogDebug("Signing transaction....")
	switch req.OutChain {
	case "eth":
		signature, err = api.keySignEth(req.OutChain, req.OutBytes)
	default:
		return fmt.Errorf("Unknown chain: %s", req.OutChain)
	}

	if err == nil {
		// Add some delay to mock TSS gen delay before sending back to Sisu server
		go func() {
			time.Sleep(time.Second * 3)
			utils.LogInfo("Sending keygen to Sisu")

			api.c.BroadcastKeySignResult(&types.KeysignResult{
				Success:        true,
				OutChain:       req.OutChain,
				OutBlockHeight: req.OutBlockHeight,
				OutHash:        req.OutHash,
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
			Signature:      signature,
		})
	}

	return err
}
