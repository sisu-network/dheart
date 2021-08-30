package core

import (
	"encoding/json"
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

	privKeys, nodes, pIDs, savedData := getEngineTestData(n)

	errCh := make(chan error)
	outCh := make(chan *p2pDataWrapper)
	engines := make([]*Engine, n)
	workId := "presign0"
	done := make(chan bool)
	finishedWorkerCount := 0

	cb := func(workerIndex int, workerId string, data []*presign.LocalPresignData) {
		finishedWorkerCount += 1

		if finishedWorkerCount == n {
			done <- true
		}
	}

	for i := 0; i < n; i++ {
		engines[i] = NewEngine(nodes[i], NewMockConnectionManager(nodes[i].PeerId.String(), outCh), helper.NewTestPresignCallback(i, cb), privKeys[i])
		engines[i].AddNodes(nodes)
	}

	// Start all engines
	for i := 0; i < n; i++ {
		request := types.NewPresignRequest(workId, n, pIDs, *savedData[i])

		go func(engine *Engine, request *types.WorkRequest, delay time.Duration) {
			// Deplay starting each engine to simluate that different workers can start at different times.
			time.Sleep(delay)
			engine.AddRequest(request)
		}(engines[i], request, time.Millisecond*time.Duration(i*350))
	}

	// Run all engines
	runEngines(engines, workId, outCh, errCh, done)
}

func runEngines(engines []*Engine, workId string, outCh chan *p2pDataWrapper, errCh chan error, done chan bool) {
	// Run all engines
	for {
		select {
		case err := <-errCh:
			panic(err)
		case <-done:
			return
		case <-time.After(time.Second * 30):
			panic(errors.New("Test timeout"))

		case p2pMsgWrapper := <-outCh:
			for _, engine := range engines {
				if engine.myNode.PeerId.String() == p2pMsgWrapper.To {
					signedMessage := &common.SignedMessage{}
					json.Unmarshal(p2pMsgWrapper.msg.Data, signedMessage)

					engine.ProcessNewMessage(signedMessage.TssMessage)
					break
				}
			}
		}
	}
}
