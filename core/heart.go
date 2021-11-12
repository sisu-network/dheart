package core

import (
	"bytes"
	"encoding/hex"
	"fmt"

	ctypes "github.com/sisu-network/cosmos-sdk/crypto/types"
	"github.com/sisu-network/dheart/client"
	htypes "github.com/sisu-network/dheart/types"

	lru "github.com/hashicorp/golang-lru"

	"github.com/sisu-network/cosmos-sdk/crypto/keys/ed25519"
	"github.com/sisu-network/cosmos-sdk/crypto/keys/secp256k1"

	"github.com/sisu-network/dheart/core/config"
	"github.com/sisu-network/dheart/db"
	"github.com/sisu-network/dheart/p2p"
	"github.com/sisu-network/dheart/utils"
	"github.com/sisu-network/dheart/worker/types"
	libchain "github.com/sisu-network/lib/chain"
	"github.com/sisu-network/lib/log"
	libCommon "github.com/sisu-network/tss-lib/common"
	"github.com/sisu-network/tss-lib/tss"
)

const (
	TX_CACHE_SIZE = 2048
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

	requestCache *lru.Cache

	requestMap map[string]interface{}
}

func NewHeart(config config.HeartConfig, client client.Client) *Heart {
	return &Heart{
		config:     config,
		aesKey:     config.AesKey,
		client:     client,
		requestMap: make(map[string]interface{}),
	}
}

func (h *Heart) Start() error {
	log.Info("Starting heart")
	// Create db
	if err := h.createDb(); err != nil {
		return err
	}

	if h.config.ShortcutPreparams {
		// Save precomputed preparams in the db. Only use this in local dev mode to speed up dev time.
		preloadPreparams(h.db, h.config)
	}

	// Cache
	requestCache, err := lru.New(TX_CACHE_SIZE)
	if err != nil {
		return err
	}
	h.requestCache = requestCache

	// Connection manager
	h.cm = p2p.NewConnectionManager(h.config.Connection)

	// Engine
	myNode := NewNode(h.privateKey.PubKey())
	h.engine = NewEngine(myNode, h.cm, h.db, h, h.privateKey)
	h.cm.AddListener(p2p.TSSProtocolID, h.engine) // Add engine to listener
	h.engine.Init()

	// Start connection manager.
	err = h.cm.Start(h.privateKey.Bytes(), h.privateKey.Type())
	if err != nil {
		log.Error("Cannot start connection manager. err =", err)
		return err
	} else {
		log.Error("Connected manager started!")
	}

	return nil
}

func (h *Heart) createDb() error {
	h.db = db.NewDatabase(&h.config.Db)
	err := h.db.Init()

	return err
}

// --- Implements Engine callback /

func (h *Heart) OnWorkKeygenFinished(result *htypes.KeygenResult) {
	h.client.PostKeygenResult(result)
}

func (h *Heart) OnWorkPresignFinished(result *htypes.PresignResult) {
	h.client.PostPresignResult(result)
}

func (h *Heart) OnWorkSigningFinished(request *types.WorkRequest, data []*libCommon.SignatureData) {
	requestKey := h.getKey("keysign", request.Chain, request.WorkId)

	value, ok := h.requestCache.Get(requestKey)
	if !ok {
		log.Critical("Cannot find client request. requestKey =", requestKey)
		h.OnWorkFailed(request, make([]*tss.PartyID, 0))
		return
	}
	clientRequest := value.(*htypes.KeysignRequest)

	// TODO: handle multiple tx here.
	signature := data[0].Signature
	if libchain.IsETHBasedChain(request.Chain) {
		signature = append(signature, data[0].SignatureRecovery[0])
	}

	result := &htypes.KeysignResult{
		Id:             clientRequest.Id,
		Success:        true,
		OutChain:       clientRequest.OutChain,
		OutBlockHeight: clientRequest.OutBlockHeight,
		OutHash:        clientRequest.OutHash,
		BytesToSign:    clientRequest.BytesToSign,
		Signature:      signature, // TODO: Support multi tx per request on Sisu
	}

	h.requestCache.Remove(requestKey)

	h.client.PostKeysignResult(result)
}

func (h *Heart) OnWorkFailed(request *types.WorkRequest, culprits []*tss.PartyID) {
	h.requestCache.Remove(h.getKey("keysign", request.Chain, request.WorkId))

	chain := request.Chain
	switch request.WorkType {
	case types.EcdsaKeygen, types.EddsaKeygen:
		result := htypes.KeygenResult{
			Chain:    chain,
			Success:  false,
			Culprits: culprits,
		}
		h.client.PostKeygenResult(&result)
	case types.EcdsaPresign, types.EddsaPresign:
		result := htypes.PresignResult{
			Chain:    chain,
			Success:  false,
			Culprits: culprits,
		}
		h.client.PostPresignResult(&result)
	case types.EcdsaSigning, types.EddsaSigning:
		result := htypes.KeysignResult{
			Success:  false,
			Culprits: culprits,
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
		log.Error("Failed to decode string, err =", err)
		return err
	}

	decrypted, err := utils.AESDecrypt(encrypted, h.aesKey)
	if err != nil {
		log.Error("Failed to decrypt key, err =", err)
		return err
	}

	if h.privateKey != nil {
		if bytes.Compare(decrypted, h.privateKey.Bytes()) != 0 {
			return fmt.Errorf("The private key has been set!")
		}

		log.Info("Private key is the same as before. Do nothing")
		return nil
	}

	switch keyType {
	case "ed25519":
		h.privateKey = &ed25519.PrivKey{Key: decrypted}
	case "secp256k1":
		h.privateKey = &secp256k1.PrivKey{Key: decrypted}
	default:
		return fmt.Errorf("Unsupported key type: %s", keyType)
	}

	err = h.Start()
	if err != nil {
		log.Error("Failed to start heart, err =", err)
	}

	return nil
}

func (h *Heart) Keygen(keygenId string, chain string, tPubKeys []ctypes.PubKey) error {
	// TODO: Check if our pubkey is one of the pubkeys.

	n := len(tPubKeys)

	nodes := NewNodes(tPubKeys)
	// For keygen, workId is the same as keygenId
	workId := keygenId
	pids := make([]*tss.PartyID, n)
	for i, node := range nodes {
		pids[i] = node.PartyId
	}

	sorted := tss.SortPartyIDs(pids)
	h.engine.AddNodes(nodes)

	request := types.NewKeygenRequest(chain, workId, len(tPubKeys), sorted, nil, n-1)
	return h.engine.AddRequest(request)
}

func (h *Heart) getKey(requestType, chain, workdId string) string {
	return fmt.Sprintf("%s__%s__%s", requestType, chain, workdId)
}

// func (h *Heart) Keysign(tx []byte, block int64, chain string, tPubKeys []ctypes.PubKey) error {
func (h *Heart) Keysign(req *htypes.KeysignRequest, tPubKeys []ctypes.PubKey) error {
	n := len(tPubKeys)

	nodes := NewNodes(tPubKeys)
	pids := make([]*tss.PartyID, n)
	for i, node := range nodes {
		pids[i] = node.PartyId
	}

	sorted := tss.SortPartyIDs(pids)
	h.engine.AddNodes(nodes)

	// TODO: Find unique workId
	workId := req.OutChain + req.OutHash
	request := types.NewSigningRequets(req.OutChain, workId, len(tPubKeys), sorted, string(req.BytesToSign))

	presignInput, err := h.db.LoadKeygenData(req.OutChain)
	if err != nil {
		return err
	}

	request.PresignInput = presignInput
	err = h.engine.AddRequest(request)

	h.requestCache.Add(h.getKey("keysign", req.OutChain, workId), req)

	return err
}

// --- End of Server API  /
