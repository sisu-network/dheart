package core

import (
	"encoding/hex"
	"fmt"

	tcrypto "github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/crypto/secp256k1"

	"github.com/sisu-network/dheart/core/config"
	"github.com/sisu-network/dheart/db"
	"github.com/sisu-network/dheart/p2p"
	"github.com/sisu-network/dheart/utils"
	libCommon "github.com/sisu-network/tss-lib/common"
	"github.com/sisu-network/tss-lib/ecdsa/keygen"
	"github.com/sisu-network/tss-lib/ecdsa/presign"
)

// The dragon heart of this component.
type Heart struct {
	config     config.HeartConfig
	db         db.Database
	cm         p2p.ConnectionManager
	engine     *Engine
	privateKey tcrypto.PrivKey
	aesKey     []byte
}

func NewHeart(config config.HeartConfig) *Heart {
	return &Heart{
		config: config,
	}
}

func (h *Heart) Start() {
	// Connection manager
	h.cm = p2p.NewConnectionManager(h.config.Connection)
	// Create db
	h.createDb()
	// Engine
	myNode := NewNode(h.privateKey.PubKey())
	h.engine = NewEngine(myNode, h.cm, h.db, h, h.privateKey)
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
}

func (h *Heart) OnWorkPresignFinished(workId string, data []*presign.LocalPresignData) {
	// TODO: implement
}

func (h *Heart) OnWorkSigningFinished(workId string, data []*libCommon.SignatureData) {
	// TODO: implement
}

// --- End fo Engien callback /

// --- Implements Server API  /

func (h *Heart) Keygen(tPubKeys []tcrypto.PubKey) {
}

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

// --- End of Server API  /
