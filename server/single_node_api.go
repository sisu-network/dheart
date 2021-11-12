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
	"github.com/sisu-network/dheart/store"
	"github.com/sisu-network/dheart/types"
	"github.com/sisu-network/lib/log"

	"github.com/ethereum/go-ethereum/ethclient"
	libchain "github.com/sisu-network/lib/chain"
)

const (
	encodedAESKey = "Jxq9PhUzvP4RZFBQivXGfA"
)

// This is a mock API to use for single localhost node. It does not have TSS signing round and
// generates a private key instead.
type SingleNodeApi struct {
	keyMap  map[string]interface{}
	store   store.Store
	ethKeys map[string]*ecdsa.PrivateKey
	c       client.Client
}

func NewSingleNodeApi(c client.Client, store store.Store) *SingleNodeApi {
	return &SingleNodeApi{
		keyMap:  make(map[string]interface{}),
		ethKeys: make(map[string]*ecdsa.PrivateKey),
		c:       c,
		store:   store,
	}
}

// Initializes private keys used for dheart
func (api *SingleNodeApi) Init() {
	// Initialized keygens
	for chain := range libchain.GetSupportedEthChains() {
		bz, err := api.store.GetEncrypted([]byte(api.getKeygenKey(chain)))
		if err != nil {
			continue
		}

		if libchain.IsETHBasedChain(chain) {
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

func (api *SingleNodeApi) KeyGen(keygenId string, chain string, tPubKeys []types.PubKeyWrapper) error {
	log.Info("keygen: chain = ", chain)
	var err error

	if libchain.IsETHBasedChain(chain) {
		err = api.keyGenEth(chain)
	} else {
		return fmt.Errorf("Unknown chain: %s", chain)
	}

	log.Info("err = ", err)
	pubKey := api.ethKeys[chain].Public()
	publicKeyECDSA, _ := pubKey.(*ecdsa.PublicKey)
	publicKeyBytes := crypto.FromECDSAPub(publicKeyECDSA)
	address := crypto.PubkeyToAddress(*publicKeyECDSA).Hex()

	if err == nil {
		// Add some delay to mock TSS gen delay before sending back to Sisu server
		go func() {
			time.Sleep(time.Second * 3)
			log.Info("Sending keygen to Sisu")

			result := types.KeygenResult{
				Chain:       chain,
				Success:     true,
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

// Key generation for ETH based chains
func (api *SingleNodeApi) keyGenEth(chain string) error {
	log.Info("Keygen for chain", chain)
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

	log.Debug("Signing transaction for chain", req.OutChain)
	if libchain.IsETHBasedChain(req.OutChain) {
		signature, err = api.keySignEth(req.OutChain, req.BytesToSign)
	} else {
		return fmt.Errorf("Unknown chain: %s", req.OutChain)
	}

	if err == nil {
		// Add some delay to mock TSS gen delay before sending back to Sisu server
		go func() {
			time.Sleep(time.Second * 3)
			log.Info("Sending Keysign to Sisu")

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
		log.Error("Cannot do key gen. Err =", err)
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
	log.Info("Deploying to chain", result.OutChain)

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
		log.Error("cannot set signatuer for tx, err =", err)
		return
	}

	client, err := ethclient.Dial(rpcEndpoint)
	if err != nil {
		log.Error("Cannot dial chain", result.OutChain, "at endpoint", rpcEndpoint)
		// TODO: Add retry mechanism here.
		return
	}

	if err := client.SendTransaction(context.Background(), signedTx); err != nil {
		log.Error("cannot dispatch tx, err =", err)
	} else {
		log.Debug("Deployment succeeded")
	}
}
