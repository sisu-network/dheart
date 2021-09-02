package helper

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"path/filepath"
	"runtime"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	libCommon "github.com/sisu-network/tss-lib/common"
	tcrypto "github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/secp256k1"

	"github.com/sisu-network/dheart/db"
	"github.com/sisu-network/dheart/types/common"
	"github.com/sisu-network/dheart/utils"
	"github.com/sisu-network/dheart/worker/types"
	"github.com/sisu-network/tss-lib/ecdsa/keygen"
	"github.com/sisu-network/tss-lib/ecdsa/presign"
	"github.com/sisu-network/tss-lib/tss"
)

const (
	TestPreparamsFixtureDirFormat  = "%s/../../data/_ecdsa_preparams_fixtures"
	TestPreparamsFixtureFileFormat = "preparams_data_%d.json"

	TestKeygenSavedDataFixtureDirFormat  = "%s/../../data/_ecdsa_keygen_saved_data_fixtures"
	TestKeygenSavedDataFixtureFileFormat = "keygen_saved_data_%d.json"

	TestPresignSavedDataFixtureDirFormat  = "%s/../../data/_ecdsa_presign_saved_data_fixtures"
	TestPresignSavedDataFixtureFileFormat = "presign_saved_data_%d.json"
)

var (
	PRIVATE_KEY_HEX = []string{
		"a817cd123ad448594a1c5259b13f11883b76c90ad084b8ebc0e5c5932850900a",
		"3e299cf28a61a5017e24709b610292237895d04d55e65d499590222833930009",
		"c5151a4068f236eb0966f24d5dad0c41391500c6d53e4cde7a9de7a1593a135d",
		"83c8ea6437a42fa9db2d2910d320b72c4134b1ab7c0a4f27e7c40b2fb85a0a8f",
		"d9a297fa63a78fa827fc82f37ed79d4cdf94223411ae2db42f28d3254a33e39d",
		"d47e57f6e4d15b19cb1cf288c04de20a2f9aac0b5f29c895663a140be083790d",
		"3d9fadefdef60a8ee9875a9ee7f3b4eff72a1ad68f0c04882a3fcd2f40b3442e",
		"f8e1890b30c8f49b2143b5a21c2dd2bfddc35be088e547c0993c6bc98fb96968",
		"0e54d4453dcd2dfb542f946fcd4efb3d13577ed59e069e37c21c06ac00da978d",
		"5622ec155232245a9a7e15f01c872be36dd46f52c4d116a64871767a43697d0a",
		"b166994aff5aaf5ac72a951e4bba28eebc7ed1060da97074d4d063367f0cbc1c",
		"9d463640fb4222358300323cd33cfe5047ae40a6783495f900bde347917f6493",
		"e45e018da02b97b44939f0ba4b609c4af4f8c8a328240fcc0365a35c45388624",
		"a00d81dd4617391ab90d60cd4f76dfe7625f26d71d8c3a5769bf2012f4269879",
		"6233130220c14e6422c414514bf64c4123aa3b6e82c33f3dfbef1907a4474635",
	}
)

type MockWorkerCallback struct {
	workerIndex     int
	keygenCallback  func(workerIndex int, request *types.WorkRequest, data []*keygen.LocalPartySaveData)
	presignCallback func(workerIndex int, request *types.WorkRequest, pids []*tss.PartyID, data []*presign.LocalPresignData)
	signingCallback func(workerIndex int, request *types.WorkRequest, data []*libCommon.SignatureData)
}

func NewTestKeygenCallback(workerIndex int, keygenCallback func(workerIndex int, request *types.WorkRequest, data []*keygen.LocalPartySaveData)) *MockWorkerCallback {
	return &MockWorkerCallback{
		workerIndex:    workerIndex,
		keygenCallback: keygenCallback,
	}
}

func NewTestPresignCallback(workerIndex int, presignCallback func(workerIndex int, request *types.WorkRequest, pids []*tss.PartyID, data []*presign.LocalPresignData)) *MockWorkerCallback {
	return &MockWorkerCallback{
		workerIndex:     workerIndex,
		presignCallback: presignCallback,
	}
}

func NewTestSigningCallback(workerIndex int, signingCallback func(workerIndex int, request *types.WorkRequest, data []*libCommon.SignatureData)) *MockWorkerCallback {
	return &MockWorkerCallback{
		workerIndex:     workerIndex,
		signingCallback: signingCallback,
	}
}

func (cb *MockWorkerCallback) OnWorkKeygenFinished(request *types.WorkRequest, data []*keygen.LocalPartySaveData) {
	cb.keygenCallback(cb.workerIndex, request, data)
}

func (cb *MockWorkerCallback) OnWorkPresignFinished(request *types.WorkRequest, pids []*tss.PartyID, data []*presign.LocalPresignData) {
	cb.presignCallback(cb.workerIndex, request, pids, data)
}

func (cb *MockWorkerCallback) OnWorkSigningFinished(request *types.WorkRequest, data []*libCommon.SignatureData) {
	cb.signingCallback(cb.workerIndex, request, data)
}

func (cb *MockWorkerCallback) OnPreExecutionFinished(request *types.WorkRequest) {
	// Do nothing.
}

func (cb *MockWorkerCallback) OnWorkFailed(request *types.WorkRequest) {
	// Do nothing.
}

func (cb *MockWorkerCallback) GetPresignData(count int, n int, pids []*tss.PartyID) []*presign.LocalPresignData {
	return nil
}

//---/

type MockEngineCallback struct {
	workerIndex     int
	keygenCallback  func(workerIndex int, workId string, data []*keygen.LocalPartySaveData)
	presignCallback func(workerIndex int, workId string, data []*presign.LocalPresignData)
	signingCallback func(workerIndex int, workId string, data []*libCommon.SignatureData)
}

func NewMockEngineKeygenCallback(workerIndex int, keygenCallback func(workerIndex int, workId string, data []*keygen.LocalPartySaveData)) *MockEngineCallback {
	return &MockEngineCallback{
		workerIndex:    workerIndex,
		keygenCallback: keygenCallback,
	}
}

func NewMockEnginePresignCallback(workerIndex int, presignCallback func(workerIndex int, workId string, data []*presign.LocalPresignData)) *MockEngineCallback {
	return &MockEngineCallback{
		workerIndex:     workerIndex,
		presignCallback: presignCallback,
	}
}

func NewMockEngineSigningCallback(workerIndex int, signingCallback func(workerIndex int, workId string, data []*libCommon.SignatureData)) *MockEngineCallback {
	return &MockEngineCallback{
		workerIndex:     workerIndex,
		signingCallback: signingCallback,
	}
}

func (cb *MockEngineCallback) OnWorkKeygenFinished(workId string, data []*keygen.LocalPartySaveData) {
	cb.keygenCallback(cb.workerIndex, workId, data)
}

func (cb *MockEngineCallback) OnWorkPresignFinished(workId string, data []*presign.LocalPresignData) {
	cb.presignCallback(cb.workerIndex, workId, data)
}

func (cb *MockEngineCallback) OnWorkSigningFinished(workId string, data []*libCommon.SignatureData) {
	cb.signingCallback(cb.workerIndex, workId, data)
}

func (cb *MockEngineCallback) OnPreExecutionFinished(workId string) {
	// Do nothing.
}

//---/

type MockDatabase struct {
	// TODO: remove this unused variable
	signingInput []*presign.LocalPresignData

	GetAvailablePresignShortFormFunc func() ([]string, []string, []int, error)
	LoadPresignFunc                  func(workIds []string, batchIndexes []int) ([]*presign.LocalPresignData, error)
}

func NewMockDatabase() db.Database {
	return &MockDatabase{}
}

func (m *MockDatabase) Init() error {
	return nil
}

func (m *MockDatabase) SaveKeygenData(chain string, workId string, pids []*tss.PartyID, keygenOutput []*keygen.LocalPartySaveData) error {
	return nil
}

func (m *MockDatabase) SavePresignData(chain string, workId string, pids []*tss.PartyID, presignOutputs []*presign.LocalPresignData) error {
	return nil
}

func (m *MockDatabase) GetAvailablePresignShortForm() ([]string, []string, []int, error) {
	if m.GetAvailablePresignShortFormFunc != nil {
		return m.GetAvailablePresignShortFormFunc()
	}

	return []string{}, []string{}, []int{}, nil
}

func (m *MockDatabase) LoadPresign(workIds []string, batchIndexes []int) ([]*presign.LocalPresignData, error) {
	if m.LoadPresignFunc != nil {
		return m.LoadPresignFunc(workIds, batchIndexes)
	}

	return nil, nil
}

func (m *MockDatabase) LoadKeygenData(chain, workId string) (*keygen.LocalPartySaveData, error) {
	return nil, nil
}

//---/

type PresignDataWrapper struct {
	Outputs [][]*presign.LocalPresignData
}

//---/

type TestDispatcher struct {
	msgCh chan *common.TssMessage
}

func NewTestDispatcher(msgCh chan *common.TssMessage) *TestDispatcher {
	return &TestDispatcher{
		msgCh: msgCh,
	}
}

//---/

func (d *TestDispatcher) BroadcastMessage(pIDs []*tss.PartyID, tssMessage *common.TssMessage) {
	d.msgCh <- tssMessage
}

// Send a message to a single destination.
func (d *TestDispatcher) UnicastMessage(dest *tss.PartyID, tssMessage *common.TssMessage) {
	d.msgCh <- tssMessage
}

//---/

func GeneratePrivateKey() tcrypto.PrivKey {
	secret := make([]byte, 32)
	rand.Read(secret)

	var priKey secp256k1.PrivKey
	priKey = secret[:32]
	return priKey
}

func GetTestPartyIds(n int) tss.SortedPartyIDs {
	if n > len(PRIVATE_KEY_HEX) {
		panic(fmt.Sprint("n is bigger than the private key array length", len(PRIVATE_KEY_HEX)))
	}

	partyIDs := make(tss.UnSortedPartyIDs, len(PRIVATE_KEY_HEX))

	for i := 0; i < len(PRIVATE_KEY_HEX); i++ {
		secret, err := hex.DecodeString(PRIVATE_KEY_HEX[i])
		if err != nil {
			panic(err)
		}
		var key secp256k1.PrivKey
		key = secret[:32]
		pubKey := key.PubKey()

		// Convert to p2p pubkey to get peer id.
		p2pPubKey, err := crypto.UnmarshalSecp256k1PublicKey(pubKey.Bytes())
		if err != nil {
			utils.LogError(err)
			return nil
		}
		peerId, err := peer.IDFromPublicKey(p2pPubKey)
		pMoniker := peerId.String()

		bigIntKey := new(big.Int).SetBytes(pubKey.Bytes())
		partyIDs[i] = tss.NewPartyID(pMoniker, pMoniker, bigIntKey)
	}

	pids := tss.SortPartyIDs(partyIDs, 0)
	pids = pids[:n]

	return pids
}

func GetTestSavedFileName(dirFormat, fileFormat string, index int) string {
	_, callerFileName, _, _ := runtime.Caller(0)
	srcDirName := filepath.Dir(callerFileName)
	fixtureDirName := fmt.Sprintf(dirFormat, srcDirName)

	return fmt.Sprintf("%s/"+fileFormat, fixtureDirName, index)
}

// --- /

func SaveTestPreparams(index int, bz []byte) error {
	fileName := GetTestSavedFileName(TestPreparamsFixtureDirFormat, TestPreparamsFixtureFileFormat, index)
	return ioutil.WriteFile(fileName, bz, 0644)
}

func LoadPreparams(index int) *keygen.LocalPreParams {
	fileName := GetTestSavedFileName(TestPreparamsFixtureDirFormat, TestPreparamsFixtureFileFormat, index)
	bz, err := ioutil.ReadFile(fileName)
	if err != nil {
		panic(err)
	}

	preparams := &keygen.LocalPreParams{}
	err = json.Unmarshal(bz, preparams)
	if err != nil {
		panic(err)
	}

	return preparams
}

func SaveKeygenOutput(data [][]*keygen.LocalPartySaveData) error {
	// We just have to save batch 0 of the outputs
	outputs := make([]*keygen.LocalPartySaveData, len(data))
	for i := range outputs {
		outputs[i] = data[i][0]
	}

	for i, output := range outputs {
		fileName := GetTestSavedFileName(TestKeygenSavedDataFixtureDirFormat, TestKeygenSavedDataFixtureFileFormat, i)

		bz, err := json.Marshal(output)
		if err != nil {
			panic(err)
		}

		if err := ioutil.WriteFile(fileName, bz, 0644); err != nil {
			return err
		}
	}

	return nil
}

// LoadKeygenSavedData loads saved data for a sorted list of party ids.
func LoadKeygenSavedData(pids tss.SortedPartyIDs) []*keygen.LocalPartySaveData {
	savedData := make([]*keygen.LocalPartySaveData, 0)

	for i := 0; i < len(PRIVATE_KEY_HEX); i++ {
		fileName := GetTestSavedFileName(TestKeygenSavedDataFixtureDirFormat, TestKeygenSavedDataFixtureFileFormat, i)

		bz, err := ioutil.ReadFile(fileName)
		if err != nil {
			panic(err)
		}

		data := &keygen.LocalPartySaveData{}
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

func SavePresignData(n int, data [][]*presign.LocalPresignData, testIndex int) error {
	wrapper := &PresignDataWrapper{
		Outputs: data,
	}

	fileName := GetTestSavedFileName(TestPresignSavedDataFixtureDirFormat, TestPresignSavedDataFixtureFileFormat, testIndex)

	bz, err := json.Marshal(wrapper)
	if err != nil {
		panic(err)
	}

	if err := ioutil.WriteFile(fileName, bz, 0644); err != nil {
		return err
	}

	return nil
}

func LoadPresignSavedData(testIndex int) *PresignDataWrapper {
	fileName := GetTestSavedFileName(TestPresignSavedDataFixtureDirFormat, TestPresignSavedDataFixtureFileFormat, testIndex)
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
