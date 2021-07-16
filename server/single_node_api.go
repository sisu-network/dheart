package server

import (
	"crypto/ecdsa"
	"encoding/base64"
	"fmt"
	"log"
	"math/big"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/sisu-network/tuktuk/client"
	"github.com/sisu-network/tuktuk/common"
	"github.com/sisu-network/tuktuk/core"
	"github.com/sisu-network/tuktuk/store"
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

func (api *SingleNodeApi) Version() string {
	return "1"
}

func (api *SingleNodeApi) KeyGen(chain string) error {
	utils.LogInfo("chain = ", chain)
	var err error
	switch chain {
	case "eth":
		err = api.keyGenEth(chain)
	}

	utils.LogInfo("err = ", err)

	if err == nil {
		// Add some delay to mock TSS gen delay before sending back to Sisu server
		go func() {
			time.Sleep(time.Second * 3)
			utils.LogInfo("Sending keygen to Sisu")
			api.c.KeygenResult(chain)
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
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		return err
	}

	api.ethKeys[chain] = privateKey

	encoded := crypto.FromECDSA(privateKey)
	return api.store.PutEncrypted(api.getKeygenKey(chain), encoded)
}

// Signing any transaction
func (api *SingleNodeApi) KeySign(chain string, serialized []byte) ([]byte, error) {
	var err error
	var data []byte

	utils.LogDebug("Signing transaction....")
	switch chain {
	case "eth":
		data, err = api.keySignEth(chain, serialized)
	}

	return data, err
}

func (api *SingleNodeApi) keySignEth(chain string, serialized []byte) ([]byte, error) {
	if api.ethKeys[chain] == nil {
		return nil, fmt.Errorf("There is no private key for this chain")
	}

	privateKey := api.ethKeys[chain]

	var tx *types.Transaction
	err := rlp.DecodeBytes(serialized, tx)
	if err != nil {
		return nil, err
	}

	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(api.chainIds[chain]), privateKey)
	if err != nil {
		log.Fatal(err)
	}

	serializedSigned, err := rlp.EncodeToBytes(signedTx)

	return serializedSigned, err
}
