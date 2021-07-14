package server

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/sisu-network/tuktuk/client"
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
	aesKey, err := base64.RawStdEncoding.DecodeString(encodedAESKey)
	if err != nil {
		panic(err)
	}

	path := os.Getenv("HOME_DIR")

	store, err := store.NewStore(path+"/apidb", aesKey)
	if err != nil {
		panic(err)
	}

	return &SingleNodeApi{
		tutuk:    tuktuk,
		keyMap:   make(map[string]interface{}),
		store:    store,
		ethKeys:  make(map[string]*ecdsa.PrivateKey),
		chainIds: initChainIds(),
		c:        c,
	}
}

func initChainIds() map[string]*big.Int {
	chainIds := make(map[string]*big.Int)
	chainIds["eth"] = big.NewInt(1)

	return chainIds
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

	if err == nil {
		// Add some delay to mock TSS gen delay before sending back to Sisu server
		go func() {
			time.Sleep(time.Second * 5)
			api.c.KeygenResult(chain)
		}()
	} else {
		utils.LogError(err)
	}

	return err
}

func (api *SingleNodeApi) keyGenEth(chain string) error {
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		return err
	}

	api.ethKeys[chain] = privateKey

	x509Encoded, _ := x509.MarshalECPrivateKey(privateKey)
	pemEncoded := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: x509Encoded})

	utils.LogInfo("Saving encrypted key...")

	return api.store.PutEncrypted([]byte(fmt.Sprintf("key_%s", chain)), []byte(pemEncoded))
}

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
