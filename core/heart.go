package core

import (
	"encoding/hex"
	"fmt"
	"time"

	ctypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/sisu-network/dheart/client"

	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"

	"github.com/sisu-network/dheart/core/config"
	"github.com/sisu-network/dheart/db"
	"github.com/sisu-network/dheart/p2p"
	types2 "github.com/sisu-network/dheart/types"
	"github.com/sisu-network/dheart/utils"
	"github.com/sisu-network/dheart/worker/types"
	"github.com/sisu-network/tss-lib/ecdsa/keygen"
	"github.com/sisu-network/tss-lib/tss"
)

// The dragon heart of this component.
type Heart struct {
	config config.HeartConfig
	db     db.Database
	cm     p2p.ConnectionManager
	engine *Engine
	client client.Client

	privateKey ctypes.PrivKey
	aesKey     []byte
}

func NewHeart(config config.HeartConfig, client client.Client) *Heart {
	return &Heart{
		config: config,
		aesKey: config.AesKey,
		client: client,
	}
}

func (h *Heart) Start() {
	utils.LogInfo("Starting heart")
	// Create db
	h.createDb()

	// Connection manager
	h.cm = p2p.NewConnectionManager(h.config.Connection)

	// Engine
	myNode := NewNode(h.privateKey.PubKey())

	h.engine = NewEngine(myNode, h.cm, h.db, h, h.privateKey)
	h.cm.AddListener(p2p.TSSProtocolID, h.engine) // Add engine to listener
	h.engine.Init()

	// Start connection manager.
	err := h.cm.Start(h.privateKey.Bytes())
	if err != nil {
		utils.LogError("Cannot start connection manager. err =", err)
	} else {
		utils.LogError("Connected manager started!")
	}
}

func (h *Heart) createDb() {
	h.db = db.NewDatabase(&h.config.Db)
	err := h.db.Init()
	if err != nil {
		panic(err)
	}
}

// --- Implements Engine callback /

func (h *Heart) OnWorkKeygenFinished(result *types2.KeygenResult) {
	h.client.PostKeygenResult(result)
}

func (h *Heart) OnWorkPresignFinished(result *types2.PresignResult) {
	h.client.PostPresignResult(result)
}

func (h *Heart) OnWorkSigningFinished(result *types2.KeysignResult) {
	h.client.PostKeysignResult(result)
}

func (h *Heart) OnWorkFailed(chain string, workType types.WorkType, culprits []*tss.PartyID) {
	switch workType {
	case types.EcdsaKeygen, types.EddsaKeygen:
		result := types2.KeygenResult{
			Chain:       chain,
			Success:     false,
			Culprits:    culprits,
		}
		h.client.PostKeygenResult(&result)
	case types.EcdsaPresign, types.EddsaPresign:
		result := types2.PresignResult{
			Chain:       chain,
			Success:     false,
			Culprits:    culprits,
		}
		h.client.PostPresignResult(&result)
	case types.EcdsaSigning, types.EddsaSigning:
		result := types2.KeysignResult{
			Success:     false,
			Culprits:    culprits,
		}
		h.client.PostKeysignResult(&result)
	}
}

// --- End fo Engine callback /

// --- Implements Server API  /

// SetPrivKey receives encrypted private key from Sisu, decrypts it and start the engine,
// network communication, etc.
func (h *Heart) SetPrivKey(encodedKey string, keyType string) error {
	encrypted, err := hex.DecodeString(encodedKey)
	if err != nil {
		return err
	}

	decrypted, err := utils.AESDecrypt(encrypted, h.aesKey)
	if err != nil {
		return err
	}

	if h.privateKey == nil {
		switch keyType {
		case "ed25519":
			h.privateKey = &ed25519.PrivKey{Key: decrypted}
		case "secp256k1":
			h.privateKey = &secp256k1.PrivKey{Key: decrypted}
		default:
			return fmt.Errorf("Unsupported key type: %s", keyType)
		}

		h.Start()
	}

	return nil
}

func (h *Heart) Keygen(keygenId string, chain string, tPubKeys []ctypes.PubKey) {
	// TODO: Check if our pubkey is one of the pubkeys.

	n := len(tPubKeys)

	nodes := NewNodes(tPubKeys)
	// For keygen, workId is the same as keygenId
	workId := keygenId
	pids := make([]*tss.PartyID, n)
	for i, node := range nodes {
		pids[i] = node.PartyId
	}

	preparams, err := h.db.LoadPreparams(chain)
	if err != nil {
		utils.LogError("Cannot load preparams. Err =", err)
		utils.LogInfo("Generating preparams...")
		preparams, err = h.generatePreparams(chain)
		if err != nil {
			// TODO Broadcast failure to Sisu using client.
			return
		}
	}

	h.engine.AddNodes(nodes)

	request := types.NewKeygenRequest(workId, len(tPubKeys), pids, *preparams, n-1)
	h.engine.AddRequest(request)
}

func (h *Heart) Keysign(txs [][]byte, block int64, chain string, tPubKeys []ctypes.PubKey) {
	n := len(tPubKeys)

	nodes := NewNodes(tPubKeys)
	pids := make([]*tss.PartyID, n)
	for i, node := range nodes {
		pids[i] = node.PartyId
	}

	// sorted := tss.SortPartyIDs(pids)
	h.engine.AddNodes(nodes)

	// Divide the txs array into multiple batches.

	// request := types.NewSigningRequets(workId, n, sorted)
}

// --- End of Server API  /

func (h *Heart) generatePreparams(chain string) (*keygen.LocalPreParams, error) {
	preparams, err := keygen.GeneratePreParams(60 * time.Second)
	if err != nil {
		utils.LogError("Cannot generate preparams. err = ", err)
		return nil, err
	}

	err = h.db.SavePreparams(chain, preparams)
	if err != nil {
		utils.LogError("Failed to save to db. err =", err)
	}

	return preparams, nil
}
