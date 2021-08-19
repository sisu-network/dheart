package ecdsa

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/sisu-network/dheart/types/common"
	"github.com/sisu-network/tss-lib/ecdsa/keygen"
	"github.com/sisu-network/tss-lib/tss"
	"github.com/stretchr/testify/assert"
)

const (
	testPreparamsFixtureDirFormat  = "%s/../../data/_ecdsa_preparams_fixtures"
	testPreparamsFixtureFileFormat = "preparams_data_%d.json"
)

//--- Miscellaneous helpers functions -- /
func processMsgWithPanicOnFail(w *KeygenWorker, tssMsg *common.TssMessage) {
	go func(w *KeygenWorker, tssMsg *common.TssMessage) {
		err := w.processNewMessage(tssMsg)
		if err != nil {
			panic(err)
		}
	}(w, tssMsg)
}

func TestKeygenEndToEnd(t *testing.T) {
	threshold := 6
	n := threshold + 1

	partyIDs := make(tss.UnSortedPartyIDs, n)
	for i := 0; i < n; i++ {
		pMoniker := fmt.Sprintf("%d", i+1)
		partyIDs[i] = tss.NewPartyID(pMoniker, pMoniker, big.NewInt(int64(i*i)+1))
		partyIDs[i].Index = i + 1
	}
	pIDs := tss.SortPartyIDs(partyIDs)

	errCh := make(chan error)
	outCh := make(chan *common.TssMessage)

	done := make(chan bool)
	finishedWorkerCount := 0

	finalOutput := make([]*keygen.LocalPartySaveData, 0)
	cb := func(data *keygen.LocalPartySaveData) {
		finalOutput = append(finalOutput, data)
		finishedWorkerCount += 1
		if finishedWorkerCount == n {
			done <- true
		}
	}

	// Generates n workers
	workers := make([]*KeygenWorker, n)
	for i := 0; i < n; i++ {
		preparams := loadPreparams(i)
		workers[i] = NewKeygenWorker(pIDs, pIDs[i], preparams, threshold, NewTestDispatcher(outCh), errCh, NewTestKeygenCallback(cb))
	}

	// Run all workers
	for i := 0; i < len(workers); i++ {
		go func(w *KeygenWorker) {
			if err := w.Start(); err != nil {
				panic(err)
			}
		}(workers[i])
	}

	// Do keygen
keygen:
	for {
		select {
		case err := <-errCh:
			panic(err)
		case <-done:
			break keygen
		case <-time.After(time.Second * 30):
			panic(errors.New("Test timeout"))

		case tssMsg := <-outCh:
			isBroadcast := tssMsg.IsBroadcast()
			if isBroadcast {
				for _, w := range workers {
					if w.myPid.Id == tssMsg.From {
						continue
					}

					processMsgWithPanicOnFail(w, tssMsg)
				}
			} else {
				if tssMsg.From == tssMsg.To {
					panic("A worker cannot send a message to itself")
				}

				for _, w := range workers {
					if w.myPid.Id == tssMsg.To {
						processMsgWithPanicOnFail(w, tssMsg)
						break
					}
				}
			}
		}
	}

	assert.Equal(t, len(finalOutput), n)
	for _, output := range finalOutput {
		// Check that everyone has the same output
		assert.Equal(t, output.ECDSAPub.X(), finalOutput[0].ECDSAPub.X())
		assert.Equal(t, output.ECDSAPub.Y(), finalOutput[0].ECDSAPub.Y())
	}
}

func generateTestPreparams(n int) {
	for i := 0; i < n; i++ {
		preParams, _ := keygen.GeneratePreParams(1 * time.Minute)
		bz, err := json.Marshal(preParams)
		if err != nil {
			panic(err)
		}

		err = saveTestPreparams(i, bz)
		if err != nil {
			panic(err)
		}
	}
}

func saveTestPreparams(index int, bz []byte) error {
	fileName := getTestPreparamsFileName(index)
	return ioutil.WriteFile(fileName, bz, 0644)
}

func getTestPreparamsFileName(index int) string {
	_, callerFileName, _, _ := runtime.Caller(0)
	srcDirName := filepath.Dir(callerFileName)
	fixtureDirName := fmt.Sprintf(testPreparamsFixtureDirFormat, srcDirName)

	return fmt.Sprintf("%s/"+testPreparamsFixtureFileFormat, fixtureDirName, index)
}

func loadPreparams(index int) *keygen.LocalPreParams {
	fileName := getTestPreparamsFileName(index)
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
