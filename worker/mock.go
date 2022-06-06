package worker

import (
	"encoding/hex"

	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"path/filepath"
	"runtime"
	"time"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/sisu-network/lib/log"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"

	"github.com/sisu-network/dheart/types/common"
	"github.com/sisu-network/dheart/worker/types"
	libCommon "github.com/sisu-network/tss-lib/common"
	eckeygen "github.com/sisu-network/tss-lib/ecdsa/keygen"
	ecsigning "github.com/sisu-network/tss-lib/ecdsa/signing"
	edkeygen "github.com/sisu-network/tss-lib/eddsa/keygen"
	"github.com/sisu-network/tss-lib/tss"
)

const (
	// Ecdsa
	TestEcPreparamsFixtureDirFormat  = "%s/../data/_ecdsa_preparams_fixtures"
	TestEcPreparamsFixtureFileFormat = "preparams_data_%d.json"

	TestEcKeygenSavedDataFixtureDirFormat  = "%s/../data/_ecdsa_keygen_saved_data_fixtures"
	TestEcKeygenSavedDataFixtureFileFormat = "keygen_saved_data_%d.json"

	TestEcPresignSavedDataFixtureDirFormat  = "%s/../data/_ecdsa_presign_saved_data_fixtures"
	TestEcPresignSavedDataFixtureFileFormat = "presign_saved_data_%d.json"

	// Eddsa
	TestEdKeygenSavedDataFixtureDirFormat  = "%s/../data/_eddsa_keygen_saved_data_fixtures"
	TestEdKeygenSavedDataFixtureFileFormat = "keygen_saved_data_%d.json"
)

var (
	PRIVATE_KEY_HEX = []string{
		"610283d69092296926deef185d7a0d8866c1399b247883d68c54a26b4c15b53b",
		"1dcc68d2b30e0f56fad4cdb510c63d72811f1d86e58cd4186be3954e6dc24d04",
		"9cdb97a75d437729811ffad23c563a4040f75073f25a4feb9251d25379ca93ae",
		"887b5ec477a9b884331feca27ec4f1d747193e9de66e5eabcf622bebb2784a41",
		"b716f8e94585d38e9b0d5f5da47c5ebaa93b9869e14169844668b48d89079a15",
		"f93e1217a26f35ff8eb0cbab2dd42a5db60f4014717c291b961bad37a15357de",
		"72d46bf8140c95301a476b4c99f443c183b20442eaee0c76c8f6432dcdb78dcd",
		"e461cf23ea4bbbdb08b2d403581a7fcd0160f87589d49a2fffe59b201c5988ea",
		"80edb8c16026ab94dfe847931f8e72d38eed925cafba27a94021b825f5ee7f5a",
		"0a8788750c9b629a36c4f5c407f8155fcc4830c13052cb09c951c771cc252072",
		"901d9358cfe84d7583f5599899e3ca8e31ccfa638a8037fa4fc7e1dfce25e451",
		"49b7f860c4c036ccb504b799ae4665aa91fdbe7348dfe7952e8d58329662213e",
		"e26f881c0bf24b8bf882eb44a89fd3658871a96cd53a7f5c8e73e8773be6db5e",
		"f83a08c326ac2fcda36e4ec6f2de3a4c3a3a5155207aea1d1154d3c0c84bf851",
		"b73455295e07b38072ac64a85c6057550f09f5e1f2e707feaa5790f719ad5084",
	}
)

type MockWorkerCallback struct {
	OnWorkerResultFunc func(request *types.WorkRequest, result *WorkerResult)

	OnNodeNotSelectedFunc    func(request *types.WorkRequest)
	OnWorkFailedFunc         func(request *types.WorkRequest)
	GetAvailablePresignsFunc func(count int, n int, allPids map[string]*tss.PartyID) ([]string, []*tss.PartyID)
	GetPresignOutputsFunc    func(presignIds []string) []*ecsigning.SignatureData_OneRoundData

	workerIndex     int
	keygenCallback  func(workerIndex int, request *types.WorkRequest, data []*eckeygen.LocalPartySaveData)
	presignCallback func(workerIndex int, request *types.WorkRequest, pids []*tss.PartyID, data []*ecsigning.SignatureData_OneRoundData)
	signingCallback func(workerIndex int, request *types.WorkRequest, data []*libCommon.ECSignature)
}

func (cb *MockWorkerCallback) OnWorkerResult(request *types.WorkRequest, result *WorkerResult) {
	if cb.OnWorkerResultFunc != nil {
		cb.OnWorkerResultFunc(request, result)
	}
}

func (cb *MockWorkerCallback) OnNodeNotSelected(request *types.WorkRequest) {
	if cb.OnNodeNotSelectedFunc != nil {
		cb.OnNodeNotSelectedFunc(request)
	}
}

func (cb *MockWorkerCallback) OnWorkFailed(request *types.WorkRequest) {
	if cb.OnWorkFailedFunc != nil {
		cb.OnWorkFailedFunc(request)
	}
}

func (cb *MockWorkerCallback) GetAvailablePresigns(count int, n int, allPids map[string]*tss.PartyID) ([]string, []*tss.PartyID) {
	if cb.GetAvailablePresignsFunc != nil {
		return cb.GetAvailablePresignsFunc(count, n, allPids)
	}

	return nil, nil
}

func (cb *MockWorkerCallback) GetPresignOutputs(presignIds []string) []*ecsigning.SignatureData_OneRoundData {
	if cb.GetPresignOutputsFunc != nil {
		return cb.GetPresignOutputsFunc(presignIds)
	}

	return nil
}

//---/

type PresignDataWrapper struct {
	KeygenOutputs []*eckeygen.LocalPartySaveData
	Outputs       []*ecsigning.SignatureData_OneRoundData
	PIDs          tss.SortedPartyIDs
}

//---/

type TestDispatcher struct {
	msgCh             chan *common.TssMessage
	preExecutionDelay time.Duration
	executionDelay    time.Duration
}

func NewTestDispatcher(msgCh chan *common.TssMessage, preExecutionDelay, executionDelay time.Duration) *TestDispatcher {
	return &TestDispatcher{
		msgCh:             msgCh,
		preExecutionDelay: preExecutionDelay,
		executionDelay:    executionDelay,
	}
}

//---/

func (d *TestDispatcher) BroadcastMessage(pIDs []*tss.PartyID, tssMessage *common.TssMessage) {
	if tssMessage.Type == common.TssMessage_UPDATE_MESSAGES {
		time.Sleep(d.executionDelay)
	} else {
		time.Sleep(d.preExecutionDelay)
	}

	d.msgCh <- tssMessage
}

// Send a message to a single destination.
func (d *TestDispatcher) UnicastMessage(dest *tss.PartyID, tssMessage *common.TssMessage) {
	if tssMessage.Type == common.TssMessage_UPDATE_MESSAGES {
		time.Sleep(d.executionDelay)
	} else {
		time.Sleep(d.preExecutionDelay)
	}

	d.msgCh <- tssMessage
}

//---/

func GetTestPartyIds(n int) tss.SortedPartyIDs {
	if n > len(PRIVATE_KEY_HEX) {
		panic(fmt.Sprint("n is bigger than the private key array length", len(PRIVATE_KEY_HEX)))
	}

	partyIDs := make(tss.UnSortedPartyIDs, len(PRIVATE_KEY_HEX))

	for i := 0; i < len(PRIVATE_KEY_HEX); i++ {
		bz, err := hex.DecodeString(PRIVATE_KEY_HEX[i])
		if err != nil {
			panic(err)
		}

		key := &secp256k1.PrivKey{Key: bz}
		pubKey := key.PubKey()

		// Convert to p2p pubkey to get peer id.
		p2pPubKey, err := crypto.UnmarshalSecp256k1PublicKey(pubKey.Bytes())
		if err != nil {
			log.Error(err)
			return nil
		}

		peerId, err := peer.IDFromPublicKey(p2pPubKey)
		if err != nil {
			log.Error(err)
			return nil
		}

		pMoniker := peerId.String()
		bigIntKey := new(big.Int).SetBytes(pubKey.Bytes())
		partyIDs[i] = tss.NewPartyID(pMoniker, pMoniker, bigIntKey)
	}

	pids := tss.SortPartyIDs(partyIDs, 0)
	pids = pids[:n]

	return pids
}

func CopySortedPartyIds(pids tss.SortedPartyIDs) tss.SortedPartyIDs {
	copy := make([]*tss.PartyID, len(pids))

	for i, p := range pids {
		copy[i] = tss.NewPartyID(p.Id, p.Moniker, p.KeyInt())
	}

	return tss.SortPartyIDs(copy)
}

func GetTestSavedFileName(dirFormat, fileFormat string, index int) string {
	_, callerFileName, _, _ := runtime.Caller(0)
	srcDirName := filepath.Dir(callerFileName)
	fixtureDirName := fmt.Sprintf(dirFormat, srcDirName)

	return fmt.Sprintf("%s/"+fileFormat, fixtureDirName, index)
}

// --- /

func SaveTestPreparams(index int, bz []byte) error {
	fileName := GetTestSavedFileName(TestEcPreparamsFixtureDirFormat, TestEcPreparamsFixtureFileFormat, index)
	return ioutil.WriteFile(fileName, bz, 0600)
}

func LoadEcPreparams(index int) *eckeygen.LocalPreParams {
	fileName := GetTestSavedFileName(TestEcPreparamsFixtureDirFormat, TestEcPreparamsFixtureFileFormat, index)
	bz, err := ioutil.ReadFile(fileName)
	if err != nil {
		panic(err)
	}

	preparams := &eckeygen.LocalPreParams{}
	err = json.Unmarshal(bz, preparams)
	if err != nil {
		panic(err)
	}

	return preparams
}

func SaveEcKeygenOutput(outputs []*eckeygen.LocalPartySaveData) error {
	for i, output := range outputs {
		fileName := GetTestSavedFileName(TestEcKeygenSavedDataFixtureDirFormat, TestEcKeygenSavedDataFixtureFileFormat, i)

		bz, err := json.Marshal(output)
		if err != nil {
			panic(err)
		}

		if err := ioutil.WriteFile(fileName, bz, 0600); err != nil {
			return err
		}
	}

	return nil
}

// LoadEcKeygenSavedData loads saved data for a sorted list of party ids.
func LoadEcKeygenSavedData(pids tss.SortedPartyIDs) []*eckeygen.LocalPartySaveData {
	savedData := make([]*eckeygen.LocalPartySaveData, 0)

	for i := 0; i < len(PRIVATE_KEY_HEX); i++ {
		fileName := GetTestSavedFileName(TestEcKeygenSavedDataFixtureDirFormat, TestEcKeygenSavedDataFixtureFileFormat, i)

		bz, err := ioutil.ReadFile(fileName)
		if err != nil {
			panic(err)
		}

		data := &eckeygen.LocalPartySaveData{}
		if err := json.Unmarshal(bz, data); err != nil {
			panic(err)
		}

		for _, pid := range pids {
			if pid.KeyInt().Cmp(data.ShareID) == 0 {
				savedData = append(savedData, data)
			}
		}
	}

	if len(savedData) != len(pids) {
		panic(fmt.Sprint("LocalSavedData array does not match ", len(savedData), len(pids)))
	}

	return savedData
}

func SaveEcPresignData(n int, keygenOutputs []*eckeygen.LocalPartySaveData, data []*ecsigning.SignatureData_OneRoundData, pIDs tss.SortedPartyIDs) error {
	wrapper := &PresignDataWrapper{
		KeygenOutputs: keygenOutputs,
		Outputs:       data,
		PIDs:          pIDs,
	}

	fileName := GetTestSavedFileName(TestEcPresignSavedDataFixtureDirFormat, TestEcPresignSavedDataFixtureFileFormat, 0)

	bz, err := json.Marshal(wrapper)
	if err != nil {
		panic(err)
	}

	if err := ioutil.WriteFile(fileName, bz, 0600); err != nil {
		return err
	}

	return nil
}

func LoadEcPresignSavedData() *PresignDataWrapper {
	fileName := GetTestSavedFileName(TestEcPresignSavedDataFixtureDirFormat, TestEcPresignSavedDataFixtureFileFormat, 0)
	bz, err := ioutil.ReadFile(fileName)
	if err != nil {
		panic(err)
	}

	wrapper := &PresignDataWrapper{}
	err = json.Unmarshal(bz, wrapper)
	if err != nil {
		panic(err)
	}

	return wrapper
}

///// Eddsa

func SaveEdKeygenOutput(data []*edkeygen.LocalPartySaveData) error {
	// We just have to save batch 0 of the outputs
	outputs := make([]*edkeygen.LocalPartySaveData, len(data))
	for i := range outputs {
		outputs[i] = data[i]
	}

	for i, output := range outputs {
		fileName := GetTestSavedFileName(TestEdKeygenSavedDataFixtureDirFormat, TestEdKeygenSavedDataFixtureFileFormat, i)

		bz, err := json.Marshal(output)
		if err != nil {
			panic(err)
		}

		if err := ioutil.WriteFile(fileName, bz, 0600); err != nil {
			return err
		}
	}

	return nil
}

func LoadEdKeygenSavedData(pids tss.SortedPartyIDs) []*edkeygen.LocalPartySaveData {
	savedData := make([]*edkeygen.LocalPartySaveData, 0)

	for i := 0; i < len(pids); i++ {
		fileName := GetTestSavedFileName(TestEdKeygenSavedDataFixtureDirFormat, TestEdKeygenSavedDataFixtureFileFormat, i)
		bz, err := ioutil.ReadFile(fileName)
		if err != nil {
			panic(err)
		}

		data := &edkeygen.LocalPartySaveData{}
		if err := json.Unmarshal(bz, data); err != nil {
			panic(err)
		}

		for _, pid := range pids {
			if pid.KeyInt().Cmp(data.ShareID) == 0 {
				savedData = append(savedData, data)
			}
		}
	}

	if len(savedData) != len(pids) {
		panic(fmt.Sprint("LocalSavedData array does not match ", len(savedData), len(pids)))
	}

	return savedData
}

/////
type MockMessageDispatcher struct {
	BroadcastMessageFunc func(pIDs []*tss.PartyID, tssMessage *common.TssMessage)
	UnicastMessageFunc   func(dest *tss.PartyID, tssMessage *common.TssMessage)
}

func (m *MockMessageDispatcher) BroadcastMessage(pIDs []*tss.PartyID, tssMessage *common.TssMessage) {
	if m.BroadcastMessageFunc != nil {
		m.BroadcastMessageFunc(pIDs, tssMessage)
	}
}

func (m *MockMessageDispatcher) UnicastMessage(dest *tss.PartyID, tssMessage *common.TssMessage) {
	if m.UnicastMessageFunc != nil {
		m.UnicastMessageFunc(dest, tssMessage)
	}
}
