package core

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strconv"
	"sync/atomic"
	"time"

	ctypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/sisu-network/dheart/client"
	htypes "github.com/sisu-network/dheart/types"

	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"

	"github.com/sisu-network/dheart/core/config"
	"github.com/sisu-network/dheart/db"
	"github.com/sisu-network/dheart/p2p"
	"github.com/sisu-network/dheart/utils"
	"github.com/sisu-network/dheart/worker/types"
	"github.com/sisu-network/lib/log"
	"github.com/sisu-network/tss-lib/tss"
)

const (
	TX_CACHE_SIZE = 2048
	RETRY_TIMEOUT = time.Second * 3
)

var (
	ErrDheartNotReady = fmt.Errorf("dheart is not ready")
)

// The dragon heart of this component.
type Heart struct {
	config     config.HeartConfig
	db         db.Database
	cm         p2p.ConnectionManager
	engine     Engine
	client     client.Client
	valPubkeys []ctypes.PubKey

	ready atomic.Value

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
	for {
		err := h.createDb()
		if err == nil {
			break
		}

		log.Error("failed to create db, err = ", err)

		time.Sleep(RETRY_TIMEOUT)
	}

	if h.config.ShortcutPreparams {
		log.Info("Loading preloaded preparams (we must be in dev mode)")
		// Save precomputed preparams in the db. Only use this in local dev mode to speed up dev time.
		preloadPreparams(h.db, h.config)
	}

	return nil
}

func (h *Heart) initConnectionManager() error {
	log.Info("Creating connection manager")

	// Connection manager
	h.cm = p2p.NewConnectionManager(h.config.Connection)

	// Engine
	myNode := NewNode(h.privateKey.PubKey())
	h.engine = NewEngine(myNode, h.cm, h.db, h, h.privateKey, NewDefaultEngineConfig())

	if h.valPubkeys != nil {
		h.engine.AddNodes(NewNodes(h.valPubkeys))
	}

	log.Info("Adding engine as listener for connection manager....")
	h.engine.Init()
	h.loadPeers(h.engine)

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

func (h *Heart) loadPeers(engine Engine) {
	peers := h.db.LoadPeers()
	if len(peers) == 0 && len(h.config.Connection.Peers) > 0 {
		// Save peers to db
		peers = h.config.Connection.Peers
		h.db.SavePeers(peers)
	}

	pubkeys := make([]ctypes.PubKey, 0)
	for _, peer := range peers {
		bz, err := hex.DecodeString(peer.PubKey)
		if err != nil {
			log.Error("loadPeers: cannot decode pubkey")
			continue
		}

		pubkey, err := utils.GetCosmosPubKey(peer.PubKeyType, bz)
		if err != nil {
			log.Error("loadPeers: get cosmos pubkey with type: ", peer.PubKeyType)
			continue
		}

		pubkeys = append(pubkeys, pubkey)
	}
	h.valPubkeys = pubkeys

	// Add pubkeys to engine
	engine.AddNodes(NewNodes(pubkeys))
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

func (h *Heart) OnWorkSigningFinished(request *types.WorkRequest, result *htypes.KeysignResult) {
	clientRequest := h.keysignRequests[request.WorkId]
	result.Request = clientRequest

	h.client.PostKeysignResult(result)
}

func (h *Heart) OnWorkFailed(request *types.WorkRequest, culprits []*tss.PartyID) {
	clientRequest := h.keysignRequests[request.WorkId]

	switch request.WorkType {
	case types.EcdsaKeygen, types.EddsaKeygen:
		result := htypes.KeygenResult{
			KeyType:  request.KeygenType,
			Outcome:  htypes.OutcomeFailure,
			Culprits: culprits,
		}
		h.client.PostKeygenResult(&result)
	case types.EcdsaPresign, types.EddsaPresign:
		result := htypes.PresignResult{
			Outcome:  htypes.OutcomeFailure,
			Culprits: culprits,
		}
		h.client.PostPresignResult(&result)
	case types.EcdsaSigning, types.EddsaSigning:
		result := htypes.KeysignResult{
			Request:  clientRequest,
			Outcome:  htypes.OutcomeFailure,
			Culprits: culprits,
		}
		h.client.PostKeysignResult(&result)
	}
}

// --- End fo Engine callback /

// --- Implements Server API  /

func (h *Heart) SetSisuReady(isReady bool) {
	log.Info("Sisu ready state = ", isReady)

	h.ready.Store(true)

	// Sisu is ready, we are now ready to process messages from network.
	h.cm.AddListener(p2p.TSSProtocolID, h.engine) // Add engine to listener
}

// TODO: remove this function
// SetPrivKey receives encrypted private key from Sisu, decrypts it and start the engine,
// network communication, etc. This is only for integration testing.
func (h *Heart) SetBootstrappedKeys(tPubKeys []ctypes.PubKey) {
	if h.valPubkeys == nil {
		h.valPubkeys = tPubKeys
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

	err = h.initConnectionManager()
	if err != nil {
		log.Error("Failed to start heart, err =", err)
	}

	return nil
}

func (h *Heart) Keygen(keygenId string, keyType string, tPubKeys []ctypes.PubKey) error {
	if h.ready.Load() != true {
		return ErrDheartNotReady
	}

	// TODO: Check if our pubkey is one of the pubkeys.
	n := len(tPubKeys)
	h.valPubkeys = tPubKeys

	nodes := NewNodes(tPubKeys)
	// For keygen, workId is the same as keygenId
	workId := keygenId
	pids := make([]*tss.PartyID, n)
	for i, node := range nodes {
		pids[i] = node.PartyId
	}
	sorted := tss.SortPartyIDs(pids)

	h.engine.AddNodes(nodes)

	request := types.NewKeygenRequest(keyType, workId, sorted, utils.GetThreshold(n), nil)

	return h.engine.AddRequest(request)
}

func (h *Heart) getKey(requestType, chain, workdId string) string {
	return fmt.Sprintf("%s__%s__%s", requestType, chain, workdId)
}

func (h *Heart) Keysign(req *htypes.KeysignRequest, tPubKeys []ctypes.PubKey) error {
	if h.ready.Load() != true {
		return ErrDheartNotReady
	}

	n := len(tPubKeys)

	nodes := NewNodes(tPubKeys)
	pids := make([]*tss.PartyID, n)
	for i, node := range nodes {
		pids[i] = node.PartyId
	}

	sorted := tss.SortPartyIDs(pids)
	h.engine.AddNodes(nodes)

	// TODO: Find unique workId
	workId := ""
	signMessages := make([]string, len(req.KeysignMessages)) // TODO: make this a byte array
	chains := make([]string, len(req.KeysignMessages))
	for i, msg := range req.KeysignMessages {
		workId = workId + msg.OutChain + msg.OutHash
		workId = utils.KeccakHash32(workId)
		signMessages[i] = string(msg.BytesToSign)
		chains[i] = msg.OutChain
	}

	// TODO: Load multiple input here.
	presignInput, err := h.db.LoadKeygenData(req.KeyType)
	if err != nil {
		return err
	}
	workRequest := types.NewSigningRequest(
		workId,
		sorted,
		utils.GetThreshold(sorted.Len()),
		signMessages,
		chains,
		presignInput,
	)

	err = h.engine.AddRequest(workRequest)

	h.keysignRequests[workRequest.WorkId] = req

	return err
}

// Called at the end of Sisu's block. This could be a time when we can check our CPU resource and
// does additional presign work.
func (h *Heart) BlockEnd(blockHeight int64) error {
	if h.ready.Load() != true {
		return ErrDheartNotReady
	}

	if h.valPubkeys == nil || len(h.valPubkeys) == 0 {
		return nil
	}

	// This operation can take time. Do it in a separate go routine and return no error immediately.
	go h.doPresign(blockHeight)

	return nil
}

// --- End of Server API  /

func (h *Heart) doPresign(blockHeight int64) {
	nodes := NewNodes(h.valPubkeys)
	pids := make([]*tss.PartyID, len(h.valPubkeys))
	for i, node := range nodes {
		pids[i] = node.PartyId
	}

	sorted := tss.SortPartyIDs(pids)

	keygenType := "ecdsa"
	presignInput, err := h.db.LoadKeygenData(keygenType)

	if err != nil {
		log.Error("Cannot get presign input, err = ", err)
	}

	if presignInput == nil {
		log.Info("Cannot find presign input. Presign cannot be executed until keygen has finished running.")
	}

	activeWorkerCount := h.engine.GetActiveWorkerCount()
	log.Verbose("activeWorkerCount = ", activeWorkerCount)

	if activeWorkerCount < MaxWorker {
		// TODO Presign work with our available worker
		workId := "presign_" + keygenType + "_" + strconv.FormatInt(blockHeight, 10)
		log.Info("Presign workId = ", workId)

		presignRequest := types.NewPresignRequest(workId, sorted, utils.GetThreshold(len(sorted)), presignInput, false, MaxBatchSize)
		err = h.engine.AddRequest(presignRequest)
		if err != nil {
			log.Error("Failed to add presign request to engine, err = ", err)
		}
	}
}
