package core

import (
	"encoding/hex"
	"fmt"
	"time"

	"github.com/sisu-network/dheart/client"
	tcrypto "github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/crypto/secp256k1"

	"github.com/sisu-network/dheart/core/config"
	"github.com/sisu-network/dheart/db"
	"github.com/sisu-network/dheart/p2p"
	"github.com/sisu-network/dheart/utils"
	"github.com/sisu-network/dheart/worker/types"
	libCommon "github.com/sisu-network/tss-lib/common"
	"github.com/sisu-network/tss-lib/ecdsa/keygen"
	"github.com/sisu-network/tss-lib/ecdsa/presign"
	"github.com/sisu-network/tss-lib/tss"
)

// The dragon heart of this component.
type Heart struct {
	config config.HeartConfig
	db     db.Database
	cm     p2p.ConnectionManager
	engine *Engine
	client client.Client

	privateKey tcrypto.PrivKey
	aesKey     []byte
}

func NewHeart(config config.HeartConfig, client client.Client) *Heart {
	return &Heart{
		config: config,
		aesKey: []byte(config.AesKey),
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

// --- Implements Engien callback /

func (h *Heart) OnWorkKeygenFinished(workId string, data []*keygen.LocalPartySaveData) {
	// TODO: implement
	h.client.PostKeygenResult(workId)
}

func (h *Heart) OnWorkPresignFinished(workId string, data []*presign.LocalPresignData) {
	// TODO: implement
}

func (h *Heart) OnWorkSigningFinished(workId string, data []*libCommon.SignatureData) {
	// TODO: implement
}

// --- End fo Engien callback /

// --- Implements Server API  /

// SisuHandshake receives encrypted private key from Sisu, decrypts it and start the engine,
// network communication, etc.
func (h *Heart) SisuHandshake(encodedKey string, keyType string) error {
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
			h.privateKey = ed25519.PrivKey(decrypted)
		case "secp256k1":
			h.privateKey = secp256k1.PrivKey(decrypted)
		default:
			return fmt.Errorf("Unsupported key type: %s", keyType)
		}

		h.Start()
	}

	return nil
}

func (h *Heart) Keygen(chain string, block int64, tPubKeys []tcrypto.PubKey) {
	n := len(tPubKeys)

	nodes := NewNodes(tPubKeys)
	workId := GetWorkId(types.ECDSA_KEYGEN, block, chain, 0, nodes)
	pids := make([]*tss.PartyID, n)
	for i, node := range nodes {
		pids[i] = node.PartyId
	}

	preparams, err := h.db.LoadPreparams(chain)
	if err != nil {
		utils.LogError("Cannot load preparams. Err =", err)
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
