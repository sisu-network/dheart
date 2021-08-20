package ecdsa

import (
	"errors"
	"fmt"
	"math/big"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/sisu-network/dheart/types/common"
	"github.com/sisu-network/dheart/worker"
	"github.com/sisu-network/tss-lib/ecdsa/keygen"
	"github.com/sisu-network/tss-lib/ecdsa/presign"
	"github.com/sisu-network/tss-lib/tss"
)

const (
	testPreparamsFixtureDirFormat  = "%s/../../data/_ecdsa_preparams_fixtures"
	testPreparamsFixtureFileFormat = "preparams_data_%d.json"

	testKeygenSavedDataFixtureDirFormat  = "%s/../../data/_ecdsa_keygen_saved_data_fixtures"
	testKeygenSavedDataFixtureFileFormat = "keygen_saved_data_%d.json"
)

type TestDispatcher struct {
	msgCh chan *common.TssMessage
}

func NewTestDispatcher(msgCh chan *common.TssMessage) *TestDispatcher {
	return &TestDispatcher{
		msgCh: msgCh,
	}
}

// Send a message to a single destination.

func (d *TestDispatcher) BroadcastMessage(pIDs []*tss.PartyID, tssMessage *common.TssMessage) {
	d.msgCh <- tssMessage
}

func (d *TestDispatcher) UnicastMessage(dest *tss.PartyID, tssMessage *common.TssMessage) {
	d.msgCh <- tssMessage
}

//---/

type TestWorkerCallback struct {
	keygenCallback  func(workerId string, data []*keygen.LocalPartySaveData)
	presignCallback func(workerId string, data []*presign.LocalPresignData)
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

func (cb *TestWorkerCallback) OnWorkKeygenFinished(workerId string, data []*keygen.LocalPartySaveData) {
	cb.keygenCallback(workerId, data)
}

func (cb *TestWorkerCallback) OnWorkPresignFinished(workerId string, data []*presign.LocalPresignData) {
	cb.presignCallback(workerId, data)
}

//---/

func getTestSavedFileName(dirFormat, fileFormat string, index int) string {
	_, callerFileName, _, _ := runtime.Caller(0)
	srcDirName := filepath.Dir(callerFileName)
	fixtureDirName := fmt.Sprintf(dirFormat, srcDirName)

	return fmt.Sprintf("%s/"+fileFormat, fixtureDirName, index)
}

func generatePartyIds(n int) tss.SortedPartyIDs {
	partyIDs := make(tss.UnSortedPartyIDs, n)
	for i := 0; i < n; i++ {
		pMoniker := fmt.Sprintf("%d", i+1)
		partyIDs[i] = tss.NewPartyID(pMoniker, pMoniker, big.NewInt(int64(i*i)+1))
		partyIDs[i].Index = i + 1
	}

	return tss.SortPartyIDs(partyIDs)
}

//---/

func startAllWorkers(workers []worker.Worker) {
	// Start all workers
	wg := sync.WaitGroup{}
	wg.Add(len(workers))
	for i := 0; i < len(workers); i++ {
		go func(w worker.Worker) {
			if err := w.Start(); err != nil {
				panic(err)
			}
			wg.Done()
		}(workers[i])
	}

	wg.Wait()
}

func runAllWorkers(workers []worker.Worker, outCh chan *common.TssMessage, errCh chan error, done chan bool) {
	for {
		select {
		case err := <-errCh:
			panic(err)
		case <-done:
			return
		case <-time.After(time.Second * 300):
			panic(errors.New("Test timeout"))

		case tssMsg := <-outCh:
			isBroadcast := tssMsg.IsBroadcast()
			if isBroadcast {
				for _, w := range workers {
					if w.GetId() == tssMsg.From {
						continue
					}

					processMsgWithPanicOnFail(w, tssMsg)
				}
			} else {
				if tssMsg.From == tssMsg.To {
					panic("A worker cannot send a message to itself")
				}

				for _, w := range workers {
					if w.GetId() == tssMsg.To {
						processMsgWithPanicOnFail(w, tssMsg)
						break
					}
				}
			}
		}
	}
}

func processMsgWithPanicOnFail(w worker.Worker, tssMsg *common.TssMessage) {
	go func(w worker.Worker, tssMsg *common.TssMessage) {
		err := w.ProcessNewMessage(tssMsg)
		if err != nil {
			panic(err)
		}
	}(w, tssMsg)
}
