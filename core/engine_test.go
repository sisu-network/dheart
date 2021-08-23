package core

import (
	"errors"
	"testing"
	"time"

	"github.com/sisu-network/dheart/types/common"
	"github.com/sisu-network/dheart/utils"
	"github.com/sisu-network/dheart/worker/helper"
	"github.com/sisu-network/dheart/worker/types"
	"github.com/sisu-network/tss-lib/ecdsa/presign"
)

func TestEngineDelayStart(t *testing.T) {
	utils.LogVerbose("Running test with tss works starting at different time.")
	n := 4

	pIDs := helper.GeneratePartyIds(n)
	savedData := helper.LoadKeygenSavedData(n)
	errCh := make(chan error)
	outCh := make(chan *common.TssMessage)
	engines := make([]*Engine, n)
	workId := "presign0"
	done := make(chan bool)
	finishedWorkerCount := 0

	cb := func(workerId string, data []*presign.LocalPresignData) {
		finishedWorkerCount += 1

		if finishedWorkerCount == n {
			done <- true
		}
	}

	for i := 0; i < n; i++ {
		engines[i] = NewEngine(pIDs[i], helper.NewTestDispatcher(outCh), helper.NewTestPresignCallback(cb))
	}

	// Start all engines
	for i := 0; i < n; i++ {
		request := types.NewPresignRequest(workId, pIDs, *savedData[i])
		go func(engine *Engine, request *types.WorkRequest, delay time.Duration) {
			// Deplay starting each engine to simluate that different workers can start at different times.
			time.Sleep(delay)
			engine.AddRequest(request)
		}(engines[i], request, time.Millisecond*time.Duration(i*350))
	}

	// Run all engines
	runEngines(engines, workId, outCh, errCh, done)
}

func runEngines(engines []*Engine, workId string, outCh chan *common.TssMessage, errCh chan error, done chan bool) {
	// Run all engines
	for {
		select {
		case err := <-errCh:
			panic(err)
		case <-done:
			return
		case <-time.After(time.Second * 10):
			panic(errors.New("Test timeout"))

		case tssMsg := <-outCh:
			isBroadcast := tssMsg.IsBroadcast()
			if isBroadcast {
				for _, engine := range engines {
					w := engine.workers[workId]
					if w != nil && engine.myPid.Id == tssMsg.From {
						continue
					}

					engine.ProcessNewMessage(tssMsg)
				}
			} else {
				if tssMsg.From == tssMsg.To {
					panic("A worker cannot send a message to itself")
				}

				for _, engine := range engines {
					if engine.myPid.Id == tssMsg.To {
						engine.ProcessNewMessage(tssMsg)
						break
					}
				}
			}
		}
	}
}
