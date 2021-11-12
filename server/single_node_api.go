package server

import (
	"context"
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

	"github.com/ethereum/go-ethereum/ethclient"
	libchain "github.com/sisu-network/lib/chain"
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

	api.chainIds = common.SUPPORTED_CHAINS_ID_MAP
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

	if libchain.IsETHBasedChain(chain) {
		err = api.keyGenEth(chain)
	} else {
		return fmt.Errorf("Unknown chain: %s", chain)
	}

	utils.LogInfo("err = ", err)
	pubKey := api.ethKeys[chain].Public()
	publicKeyECDSA, _ := pubKey.(*ecdsa.PublicKey)
	publicKeyBytes := crypto.FromECDSAPub(publicKeyECDSA)
	address := crypto.PubkeyToAddress(*publicKeyECDSA).Hex()

	if err == nil {
		// Add some delay to mock TSS gen delay before sending back to Sisu server
		go func() {
			time.Sleep(time.Second * 3)
			utils.LogInfo("Sending keygen to Sisu")

			result := types.KeygenResult{
				Chain:       chain,
				Success:     true,
				PubKeyBytes: publicKeyBytes,
				Address:     address,
			}
			if err := api.c.PostKeygenResult(&result); err != nil {
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

func (api *SingleNodeApi) keySignEth(chain string, bytesToSign []byte) ([]byte, error) {
	if api.ethKeys[chain] == nil {
		return nil, fmt.Errorf("There is no private key for this chain")
	}

	privateKey := api.ethKeys[chain]
	sig, err := crypto.Sign(bytesToSign, privateKey)

	return sig, err
}

// Signing any transaction
func (api *SingleNodeApi) KeySign(req *types.KeysignRequest, tPubKeys []types.PubKeyWrapper) error {
	var err error
	var signature []byte

	utils.LogDebug("Signing transaction for chain", req.OutChain)
	if libchain.IsETHBasedChain(req.OutChain) {
		signature, err = api.keySignEth(req.OutChain, req.BytesToSign)
	} else {
		return fmt.Errorf("Unknown chain: %s", req.OutChain)
	}

	if err == nil {
		// Add some delay to mock TSS gen delay before sending back to Sisu server
		go func() {
			time.Sleep(time.Second * 3)
			utils.LogInfo("Sending Keysign to Sisu")

			result := &types.KeysignResult{
				Id:             req.Id,
				Success:        true,
				OutChain:       req.OutChain,
				OutBlockHeight: req.OutBlockHeight,
				OutHash:        req.OutHash,
				BytesToSign:    req.BytesToSign,
				Signature:      signature,
			}

			// api.deployToChain(result)
			api.c.PostKeysignResult(result)
		}()
	} else {
		utils.LogError("Cannot do key gen. Err =", err)
		api.c.PostKeysignResult(&types.KeysignResult{
			Id:             req.Id,
			Success:        false,
			ErrMesage:      err.Error(),
			OutChain:       req.OutChain,
			OutBlockHeight: req.OutBlockHeight,
			OutHash:        req.OutHash,
			BytesToSign:    req.BytesToSign,
			Signature:      signature,
		})
	}

	return err
}

func (api *SingleNodeApi) SetPrivKey(encodedKey string, keyType string) error {
	return nil
}

// Used for debugging only.
func (api *SingleNodeApi) deployToChain(result *types.KeysignResult) {
	utils.LogInfo("Deploying to chain", result.OutChain)

	chainId := big.NewInt(1)
	rpcEndpoint := "http://localhost:7545"

	if result.OutChain == "sisu-eth" {
		chainId = big.NewInt(36767)
		rpcEndpoint = "http://localhost:8545"
	}

	tx := &eTypes.Transaction{}
	tx.UnmarshalBinary(result.BytesToSign)

	signedTx, err := tx.WithSignature(eTypes.NewEIP155Signer(chainId), result.Signature)
	if err != nil {
		utils.LogError("cannot set signatuer for tx, err =", err)
		return
	}

	client, err := ethclient.Dial(rpcEndpoint)
	if err != nil {
		utils.LogError("Cannot dial chain", result.OutChain, "at endpoint", rpcEndpoint)
		// TODO: Add retry mechanism here.
		return
	}

	if err := client.SendTransaction(context.Background(), signedTx); err != nil {
		utils.LogError("cannot dispatch tx, err =", err)
	} else {
		utils.LogDebug("Deployment succeeded")
	}
}
