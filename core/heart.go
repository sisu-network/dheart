package core

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strconv"

	ctypes "github.com/sisu-network/cosmos-sdk/crypto/types"
	"github.com/sisu-network/dheart/client"
	htypes "github.com/sisu-network/dheart/types"

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
	config   config.HeartConfig
	db       db.Database
	cm       p2p.ConnectionManager
	engine   Engine
	client   client.Client
	tPubKeys []ctypes.PubKey

	privateKey ctypes.PrivKey
	aesKey     []byte

	keysignRequests map[string]*htypes.KeysignRequest
}

func NewHeart(config config.HeartConfig, client client.Client) *Heart {
	return &Heart{
		config:          config,
		aesKey:          config.AesKey,
		client:          client,
		keysignRequests: make(map[string]*htypes.KeysignRequest),
	}
}

func (h *Heart) Start() error {
	log.Info("Starting heart")
	// Create db
	if err := h.createDb(); err != nil {
		return err
	}

	if h.config.ShortcutPreparams {
		log.Info("Loading preloaded preparams (we must be in dev mode)")
		// Save precomputed preparams in the db. Only use this in local dev mode to speed up dev time.
		preloadPreparams(h.db, h.config)
	}

	// Connection manager
	h.cm = p2p.NewConnectionManager(h.config.Connection)

	// Engine
	myNode := NewNode(h.privateKey.PubKey())
	h.engine = NewEngine(myNode, h.cm, h.db, h, h.privateKey, NewDefaultEngineConfig())
	if h.tPubKeys != nil {
		h.engine.AddNodes(NewNodes(h.tPubKeys))
	}

	log.Info("Adding engine as listener for connection manager....")

	h.cm.AddListener(p2p.TSSProtocolID, h.engine) // Add engine to listener
	h.engine.Init()

	// Start connection manager.
	err := h.cm.Start(h.privateKey.Bytes(), h.privateKey.Type())
	if err != nil {
		log.Error("Cannot start connection manager. err =", err)
		return err
	} else {
		log.Info("Connected manager started!")
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
	clientRequest := h.keysignRequests[request.WorkId]

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

	h.client.PostKeysignResult(result)
}

func (h *Heart) OnWorkFailed(request *types.WorkRequest, culprits []*tss.PartyID) {
	chain := request.Chain
	switch request.WorkType {
	case types.EcdsaKeygen, types.EddsaKeygen:
		result := htypes.KeygenResult{
			KeyType:  libchain.GetKeygenType(request.Chain),
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

func (h *Heart) SetBootstrappedKeys(tPubKeys []ctypes.PubKey) {
	if h.tPubKeys == nil {
		h.tPubKeys = tPubKeys
	}
}

func (h *Heart) SetPrivKey(encryptedKey string, tendermintKeyType string) error {
	encrypted, err := hex.DecodeString(encryptedKey)
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

	switch tendermintKeyType {
	case "ed25519":
		h.privateKey = &ed25519.PrivKey{Key: decrypted}
	case "secp256k1":
		h.privateKey = &secp256k1.PrivKey{Key: decrypted}
	default:
		return fmt.Errorf("Unsupported key type: %s", tendermintKeyType)
	}

	err = h.Start()
	if err != nil {
		log.Error("Failed to start heart, err =", err)
	}

	return nil
}

func (h *Heart) Keygen(keygenId string, keyType string, tPubKeys []ctypes.PubKey) error {
	// TODO: Check if our pubkey is one of the pubkeys.

	n := len(tPubKeys)
	h.tPubKeys = tPubKeys

	nodes := NewNodes(tPubKeys)
	// For keygen, workId is the same as keygenId
	workId := keygenId
	pids := make([]*tss.PartyID, n)
	for i, node := range nodes {
		pids[i] = node.PartyId
	}
	sorted := tss.SortPartyIDs(pids)

	h.engine.AddNodes(nodes)

	request := types.NewKeygenRequest(keyType, workId, len(tPubKeys), sorted, nil, n-1)
	return h.engine.AddRequest(request)
}

func (h *Heart) getKey(requestType, chain, workdId string) string {
	return fmt.Sprintf("%s__%s__%s", requestType, chain, workdId)
}

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
	workRequest := types.NewSigningRequets(req.OutChain, workId, len(tPubKeys), sorted, string(req.BytesToSign))

	presignInput, err := h.db.LoadKeygenData(libchain.GetKeyTypeForChain(req.OutChain))
	if err != nil {
		return err
	}

	workRequest.PresignInput = presignInput
	err = h.engine.AddRequest(workRequest)

	h.keysignRequests[workRequest.WorkId] = req

	return err
}

// Called at the end of Sisu's block. This could be a time when we can check our CPU resource and
// does additional presign work.
func (h *Heart) BlockEnd(blockHeight int64) error {
	if h.tPubKeys == nil || len(h.tPubKeys) == 0 {
		return nil
	}

	nodes := NewNodes(h.tPubKeys)
	pids := make([]*tss.PartyID, len(h.tPubKeys))
	for i, node := range nodes {
		pids[i] = node.PartyId
	}

	sorted := tss.SortPartyIDs(pids)

	keygenType := "ecdsa"
	presignInput, err := h.db.LoadKeygenData(keygenType)

	if err != nil {
		log.Error("Cannot get presign input, err = ", err)
		return err
	}

	if presignInput == nil {
		log.Info("Cannot find presign input. Presign cannot be executed until keygen has finished running.")
		return nil
	}

	activeWorkerCount := h.engine.GetActiveWorkerCount()
	log.Verbose("activeWorkerCount = ", activeWorkerCount)

	if activeWorkerCount < MaxWorker {
		// TODO Presign work with our available worker
		workId := "presign_" + keygenType + "_" + strconv.FormatInt(blockHeight, 10)
		log.Info("Presign workId = ", workId)

		presignRequest := types.NewPresignRequest(workId, len(h.tPubKeys), sorted, presignInput, false)
		err = h.engine.AddRequest(presignRequest)
		if err != nil {
			log.Error("Failed to add presign request to engine, err = ", err)
		}
	}

	return nil
}

// --- End of Server API  /
