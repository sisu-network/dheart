package main

import (
	"crypto/ecdsa"
	"encoding/hex"

	"math/rand"

	ipfslog "github.com/ipfs/go-log"

	"crypto/elliptic"
	"flag"
	"fmt"
	"math/big"
	"time"

	thelper "github.com/sisu-network/dheart/test/e2e/helper"
	"github.com/sisu-network/dheart/utils"
	"github.com/sisu-network/dheart/worker"
	libchain "github.com/sisu-network/lib/chain"

	"github.com/sisu-network/dheart/core"
	"github.com/sisu-network/dheart/core/config"
	"github.com/sisu-network/dheart/db"
	"github.com/sisu-network/dheart/p2p"
	htypes "github.com/sisu-network/dheart/types"
	"github.com/sisu-network/dheart/worker/types"
	"github.com/sisu-network/lib/log"
	"github.com/sisu-network/tss-lib/tss"

	ctypes "github.com/cosmos/cosmos-sdk/crypto/types"
)

type EngineCallback struct {
	keygenDataCh  chan *htypes.KeygenResult
	presignDataCh chan *htypes.PresignResult
	signingDataCh chan *htypes.KeysignResult
}

func NewEngineCallback(
	keygenDataCh chan *htypes.KeygenResult,
	presignDataCh chan *htypes.PresignResult,
	signingDataCh chan *htypes.KeysignResult,
) *EngineCallback {
	return &EngineCallback{
		keygenDataCh, presignDataCh, signingDataCh,
	}
}

func (cb *EngineCallback) OnWorkKeygenFinished(result *htypes.KeygenResult) {
	cb.keygenDataCh <- result
}

func (cb *EngineCallback) OnWorkPresignFinished(result *htypes.PresignResult) {
	cb.presignDataCh <- result
}

func (cb *EngineCallback) OnWorkSigningFinished(request *types.WorkRequest, result *htypes.KeysignResult) {
	if cb.signingDataCh != nil {
		cb.signingDataCh <- result
	}
}

func (cb *EngineCallback) OnWorkFailed(request *types.WorkRequest, culprits []*tss.PartyID) {
	if cb.signingDataCh != nil {
		cb.signingDataCh <- nil
	}
}

func getSortedPartyIds(n int) tss.SortedPartyIDs {
	keys := p2p.GetAllSecp256k1PrivateKeys(n)
	partyIds := make([]*tss.PartyID, n)

	// Creates list of party ids
	for i := 0; i < n; i++ {
		bz := keys[i].PubKey().Bytes()
		peerId := p2p.P2PIDFromKey(keys[i])
		party := tss.NewPartyID(peerId.String(), "", new(big.Int).SetBytes(bz))
		partyIds[i] = party
	}

	return tss.SortPartyIDs(partyIds, 0)
}

func getDb(index int) db.Database {
	dbConfig := config.GetLocalhostDbConfig()
	dbConfig.Schema = fmt.Sprintf("dheart%d", index)
	dbConfig.InMemory = true

	dbInstance := db.NewDatabase(&dbConfig)

	err := dbInstance.Init()
	if err != nil {
		panic(err)
	}

	return dbInstance
}

func doKeygen(pids tss.SortedPartyIDs, index int, engine core.Engine, outCh chan *htypes.KeygenResult) *htypes.KeygenResult {
	// Add request
	workId := "keygen0"
	threshold := utils.GetThreshold(len(pids))
	request := types.NewKeygenRequest("ecdsa", workId, pids, threshold, worker.LoadEcPreparams(index))
	err := engine.AddRequest(request)
	if err != nil {
		panic(err)
	}

	var result *htypes.KeygenResult
	select {
	case result = <-outCh:
	case <-time.After(time.Second * 100):
		panic("Keygen timeout")
	}

	return result
}

func verifySignature(pubkey *ecdsa.PublicKey, msg string, R, S *big.Int) {
	ok := ecdsa.Verify(pubkey, []byte(msg), R, S)
	if !ok {
		panic(fmt.Sprintf("Signature verification fails for msg: %s", msg))
	}
}

func testKeysign(database db.Database, pids []*tss.PartyID, engine core.Engine, keysignch chan *htypes.KeysignResult,
	keygenResult *htypes.KeygenResult, message []byte) {
	workId := "keysign"
	messages := []string{string(message)}
	chains := []string{"eth", "eth"}

	presignInput, err := database.LoadKeygenData(libchain.KEY_TYPE_ECDSA)
	if err != nil {
		panic(err)
	}

	threshold := utils.GetThreshold(len(pids))
	request := types.NewSigningRequest(workId, pids, threshold, messages, chains, presignInput)

	err = engine.AddRequest(request)
	if err != nil {
		panic(err)
	}

	var result *htypes.KeysignResult
	select {
	case result = <-keysignch:
	case <-time.After(time.Second * 100):
		panic("Signing timeout")
	}

	if result == nil {
		panic("result is nil")
	}

	switch result.Outcome {
	case htypes.OutcomeSuccess:
		for i, msg := range messages {
			x, y := elliptic.Unmarshal(tss.EC(tss.EcdsaScheme), keygenResult.PubKeyBytes)
			pk := ecdsa.PublicKey{
				Curve: tss.EC(tss.EcdsaScheme),
				X:     x,
				Y:     y,
			}

			sig := result.Signatures[i]
			if len(sig) != 65 {
				log.Info("Signature hex = ", hex.EncodeToString(sig))
				panic(fmt.Sprintf("Signature length is not correct. actual length = %d", len(sig)))
			}
			sig = sig[:64]

			r := sig[:32]
			s := sig[32:]

			verifySignature(&pk, msg, new(big.Int).SetBytes(r), new(big.Int).SetBytes(s))
		}

		log.Info("Signing succeeded!")

	case htypes.OutcomeFailure:
		panic("Failed to create signature")
	case htypes.OutcometNotSelected:
		log.Info("Node is not selected.")
	}
}

func main() {
	ipfslog.SetLogLevel("dheart", "debug")

	var index, n, seed int
	var isSlow bool
	flag.IntVar(&index, "index", 0, "listening port")
	flag.IntVar(&n, "n", 2, "number of nodes in the test")
	flag.BoolVar(&isSlow, "is-slow", false, "Use it when testing message caching mechanism")
	flag.IntVar(&seed, "seed", 0, "seed for the test")
	flag.Parse()

	cfg, privateKey := p2p.GetMockSecp256k1Config(n, index)
	cm := p2p.NewConnectionManager(cfg)
	if isSlow {
		cm = thelper.NewSlowConnectionManager(cfg)
	}
	err := cm.Start(privateKey, "secp256k1")
	if err != nil {
		panic(err)
	}

	pids := make([]*tss.PartyID, n)
	allKeys := p2p.GetAllSecp256k1PrivateKeys(n)
	nodes := make([]*core.Node, n)
	tendermintPubKeys := make([]ctypes.PubKey, n)

	// Add nodes
	privKeys := p2p.GetAllSecp256k1PrivateKeys(n)
	for i := 0; i < n; i++ {
		pubKey := privKeys[i].PubKey()
		node := core.NewNode(pubKey)
		nodes[i] = node
		pids[i] = node.PartyId
	}
	pids = tss.SortPartyIDs(pids)

	// Create new engine
	keygenCh := make(chan *htypes.KeygenResult)
	keysignch := make(chan *htypes.KeysignResult)
	cb := NewEngineCallback(keygenCh, nil, keysignch)
	database := getDb(index)

	engine := core.NewEngine(nodes[index], cm, database, cb, allKeys[index], config.NewDefaultTimeoutConfig())
	cm.AddListener(p2p.TSSProtocolID, engine)

	// Add nodes
	for i := 0; i < n; i++ {
		engine.AddNodes(nodes)
		tendermintPubKeys[i] = privKeys[i].PubKey()
	}

	time.Sleep(time.Second * 3)

	// Keygen
	keygenResult := doKeygen(pids, index, engine, keygenCh)

	// Keysign
	log.Info("Doing keysign now!")
	rand.Seed(int64(seed + 110))
	for i := 0; i < 20; i++ {
		msg := make([]byte, 20)
		rand.Read(msg) //nolint
		if err != nil {
			panic(err)
		}
		log.Info("Msg hex = ", hex.EncodeToString(msg))
		testKeysign(database, pids, engine, keysignch, keygenResult, msg)
	}
}
