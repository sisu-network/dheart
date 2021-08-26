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

	"github.com/sisu-network/dheart/types/common"
	"github.com/sisu-network/dheart/utils"
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

type TestWorkerCallback struct {
	keygenCallback  func(workerId string, data []*keygen.LocalPartySaveData)
	presignCallback func(workerId string, data []*presign.LocalPresignData)
	signingCallback func(workerId string, data []*libCommon.SignatureData)
}

func NewTestKeygenCallback(keygenCallback func(workerId string, data []*keygen.LocalPartySaveData)) *TestWorkerCallback {
	return &TestWorkerCallback{
		keygenCallback: keygenCallback,
	}
}

func NewTestPresignCallback(presignCallback func(workerId string, data []*presign.LocalPresignData)) *TestWorkerCallback {
	return &TestWorkerCallback{
		presignCallback: presignCallback,
	}
}

func NewTestSigningCallback(signingCallback func(workerId string, data []*libCommon.SignatureData)) *TestWorkerCallback {
	return &TestWorkerCallback{
		signingCallback: signingCallback,
	}
}

func (cb *TestWorkerCallback) OnWorkKeygenFinished(workerId string, data []*keygen.LocalPartySaveData) {
	cb.keygenCallback(workerId, data)
}

func (cb *TestWorkerCallback) OnWorkPresignFinished(workerId string, data []*presign.LocalPresignData) {
	cb.presignCallback(workerId, data)
}

func (cb *TestWorkerCallback) OnWorkSigningFinished(workerId string, data []*libCommon.SignatureData) {
	cb.signingCallback(workerId, data)
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

func GeneratePartyIds(n int) tss.SortedPartyIDs {
	partyIDs := make(tss.UnSortedPartyIDs, n)
	for i := 0; i < n; i++ {
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

		pMoniker := fmt.Sprintf("%d", i+1)
		partyIDs[i] = tss.NewPartyID(peerId.String(), pMoniker, new(big.Int).SetBytes(pubKey.Bytes()))
		partyIDs[i].Index = i + 1
	}

	return tss.SortPartyIDs(partyIDs)
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

func LoadKeygenSavedData(n int) []*keygen.LocalPartySaveData {
	savedData := make([]*keygen.LocalPartySaveData, n)

	for i := 0; i < n; i++ {
		fileName := GetTestSavedFileName(TestKeygenSavedDataFixtureDirFormat, TestKeygenSavedDataFixtureFileFormat, i)

		bz, err := ioutil.ReadFile(fileName)
		if err != nil {
			panic(err)
		}

		data := &keygen.LocalPartySaveData{}
		if err := json.Unmarshal(bz, data); err != nil {
			panic(err)
		}

		savedData[i] = data
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
