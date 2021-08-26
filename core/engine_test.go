package core

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/sisu-network/dheart/p2p"
	"github.com/sisu-network/dheart/types/common"
	"github.com/sisu-network/dheart/utils"
	"github.com/sisu-network/dheart/worker/helper"
	"github.com/sisu-network/dheart/worker/types"
	"github.com/sisu-network/tss-lib/ecdsa/presign"
)

func TestEngineDelayStart(t *testing.T) {
	utils.LogVerbose("Running test with tss works starting at different time.")
	n := 4

	privKeys, nodes, pIDs, savedData := getEngineTestData(n)

	errCh := make(chan error)
	outCh := make(chan *p2p.P2PMessage)
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
		engines[i] = NewEngine(pIDs[i], NewMockConnectionManager(outCh), helper.NewTestPresignCallback(cb), privKeys[i])
		engines[i].AddNodes(nodes)
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

func runEngines(engines []*Engine, workId string, outCh chan *p2p.P2PMessage, errCh chan error, done chan bool) {
	// Run all engines
	for {
		select {
		case err := <-errCh:
			panic(err)
		case <-done:
			return
		case <-time.After(time.Second * 30):
			panic(errors.New("Test timeout"))

		case p2pMsg := <-outCh:
			signedMessage := &common.SignedMessage{}
			json.Unmarshal(p2pMsg.Data, signedMessage)
			tssMsg := signedMessage.TssMessage

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
