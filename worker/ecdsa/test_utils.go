package ecdsa

import (
	"errors"
	"sync"
	"time"

	"github.com/sisu-network/dheart/types/common"
	"github.com/sisu-network/dheart/worker"
	libCommon "github.com/sisu-network/tss-lib/common"
	"github.com/sisu-network/tss-lib/ecdsa/keygen"
	"github.com/sisu-network/tss-lib/ecdsa/presign"
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

func startAllWorkers(workers []worker.Worker) {
	// Start all workers
	wg := sync.WaitGroup{}
	wg.Add(len(workers))
	for i := 0; i < len(workers); i++ {
		go func(w worker.Worker) {
			if err := w.Start(make([]*common.TssMessage, 0)); err != nil {
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
					if w.GetPartyId() == tssMsg.From {
						continue
					}

					processMsgWithPanicOnFail(w, tssMsg)
				}
			} else {
				if tssMsg.From == tssMsg.To {
					panic("A worker cannot send a message to itself")
				}

				for _, w := range workers {
					if w.GetPartyId() == tssMsg.To {
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
