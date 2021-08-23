package helper

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"path/filepath"
	"runtime"

	"github.com/sisu-network/dheart/types/common"
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

func GeneratePartyIds(n int) tss.SortedPartyIDs {
	partyIDs := make(tss.UnSortedPartyIDs, n)
	for i := 0; i < n; i++ {
		pMoniker := fmt.Sprintf("%d", i+1)
		partyIDs[i] = tss.NewPartyID(pMoniker, pMoniker, big.NewInt(int64(i*i)+1))
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

func SaveKeysignOutput(outputs []*keygen.LocalPartySaveData) error {
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
