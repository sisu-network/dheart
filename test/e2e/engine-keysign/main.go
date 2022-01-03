package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"database/sql"
	"flag"
	"fmt"
	"math/big"
	"time"

	libchain "github.com/sisu-network/lib/chain"

	"github.com/sisu-network/dheart/core"
	"github.com/sisu-network/dheart/core/config"
	"github.com/sisu-network/dheart/db"
	"github.com/sisu-network/dheart/p2p"
	htypes "github.com/sisu-network/dheart/types"
	"github.com/sisu-network/dheart/worker/helper"
	"github.com/sisu-network/dheart/worker/types"
	"github.com/sisu-network/lib/log"
	"github.com/sisu-network/tss-lib/tss"

	ctypes "github.com/sisu-network/cosmos-sdk/crypto/types"
	libCommon "github.com/sisu-network/tss-lib/common"
)

type EngineCallback struct {
	keygenDataCh  chan *htypes.KeygenResult
	presignDataCh chan *htypes.PresignResult
	signingDataCh chan []*libCommon.SignatureData
}

func NewEngineCallback(
	keygenDataCh chan *htypes.KeygenResult,
	presignDataCh chan *htypes.PresignResult,
	signingDataCh chan []*libCommon.SignatureData,
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

func (cb *EngineCallback) OnWorkSigningFinished(request *types.WorkRequest, data []*libCommon.SignatureData) {
	if cb.signingDataCh != nil {
		cb.signingDataCh <- data
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

func resetDb(dbConfig config.DbConfig) error {
	database, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", dbConfig.Username, dbConfig.Password, dbConfig.Host, dbConfig.Port, dbConfig.Schema))
	if err != nil {
		return err
	}

	database.Exec("DROP TABLE keygen")
	database.Exec("DROP TABLE keysign")
	database.Exec("DROP TABLE schema_migrations")

	return nil
}

func getDb(index int) db.Database {
	dbConfig := config.GetLocalhostDbConfig()
	dbConfig.Schema = fmt.Sprintf("dheart%d", index)
	dbConfig.MigrationPath = "file://../../../db/migrations/"

	resetDb(dbConfig)

	dbInstance := db.NewDatabase(&dbConfig)

	err := dbInstance.Init()
	if err != nil {
		panic(err)
	}

	return dbInstance
}

func keygen(pids tss.SortedPartyIDs, index int, engine core.Engine, outCh chan *htypes.KeygenResult) *htypes.KeygenResult {
	n := len(pids)

	// Add request
	workId := "keygen0"
	request := types.NewKeygenRequest("ecdsa", workId, n, pids, helper.LoadPreparams(index), n-1)
	err := engine.AddRequest(request)
	if err != nil {
		panic(err)
	}

	var result *htypes.KeygenResult
	select {
	case result = <-outCh:
	case <-time.After(time.Second * 20):
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

func main() {
	var index, n int
	flag.IntVar(&index, "index", 0, "listening port")
	flag.Parse()

	n = 2

	config, privateKey := p2p.GetMockSecp256k1Config(n, index)
	cm := p2p.NewConnectionManager(config)
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
	keysignch := make(chan []*libCommon.SignatureData)
	cb := NewEngineCallback(keygenCh, nil, keysignch)
	database := getDb(index)

	engine := core.NewEngine(nodes[index], cm, database, cb, allKeys[index], core.NewDefaultEngineConfig())
	cm.AddListener(p2p.TSSProtocolID, engine)

	// Add nodes
	for i := 0; i < n; i++ {
		engine.AddNodes(nodes)
		tendermintPubKeys[i] = privKeys[i].PubKey()
	}

	time.Sleep(time.Second * 3)

	// Keygen
	keygenResult := keygen(pids, index, engine, keygenCh)
	log.Info("Doing keysign now!")

	// Keysign
	workId := "keysign"
	messages := []string{"First message", "second message"}

	request := types.NewSigningRequest(workId, n, pids, messages)
	presignInput, err := database.LoadKeygenData(libchain.KEY_TYPE_ECDSA)
	if err != nil {
		panic(err)
	}
	request.PresignInput = presignInput

	err = engine.AddRequest(request)
	if err != nil {
		panic(err)
	}

	var signatures []*libCommon.SignatureData
	select {
	case signatures = <-keysignch:
	case <-time.After(time.Second * 20):
		panic("Keygen timeout")
	}

	for i, msg := range messages {
		x, y := elliptic.Unmarshal(tss.EC(), keygenResult.PubKeyBytes)
		pk := ecdsa.PublicKey{
			Curve: tss.EC(),
			X:     x,
			Y:     y,
		}

		verifySignature(&pk, msg, new(big.Int).SetBytes(signatures[i].R), new(big.Int).SetBytes(signatures[i].S))
	}

	log.Info("Verification succeeded!")
}
