package core

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	htypes "github.com/sisu-network/dheart/types"
	"github.com/sisu-network/dheart/types/common"
	"github.com/sisu-network/dheart/worker/helper"
	"github.com/sisu-network/dheart/worker/types"
	"github.com/sisu-network/lib/log"
	"github.com/sisu-network/tss-lib/tss"
	"github.com/stretchr/testify/require"
)

func TestEngineDelayStart(t *testing.T) {
	log.Verbose("Running test with tss works starting at different time.")
	n := 4

	privKeys, nodes, pIDs, savedData := getEngineTestData(n)

	errCh := make(chan error)
	outCh := make(chan *p2pDataWrapper)
	engines := make([]Engine, n)
	workId := "presign0"
	done := make(chan bool)
	finishedWorkerCount := 0
	outputLock := &sync.Mutex{}
	pidString := ""
	presignIds := make([]string, n)
	pidStrings := make([]string, n)
	for i := range presignIds {
		presignIds[i] = fmt.Sprintf("%s-%d", workId, i)
		pidString = pidString + pIDs[i].Id
		if i < n-1 {
			pidString = pidString + ","
		}
	}
	for i := range presignIds {
		pidStrings[i] = pidString
	}

	for i := 0; i < n; i++ {
		cb := func(result *htypes.PresignResult) {
			outputLock.Lock()
			defer outputLock.Unlock()

			finishedWorkerCount += 1
			if finishedWorkerCount == n {
				done <- true
			}
		}

		engines[i] = NewEngine(
			nodes[i],
			NewMockConnectionManager(nodes[i].PeerId.String(), outCh),
			getMokDbForAvailManager(presignIds, pidStrings),
			&helper.MockEngineCallback{
				OnWorkPresignFinishedFunc: cb,
			},
			privKeys[i],
			NewDefaultEngineConfig(),
		)
		engines[i].AddNodes(nodes)
	}

	// Start all engines
	for i := 0; i < n; i++ {
		request := types.NewPresignRequest(workId, n, helper.CopySortedPartyIds(pIDs), savedData[i], true)

		go func(engine Engine, request *types.WorkRequest, delay time.Duration) {
			// Deplay starting each engine to simulate that different workers can start at different times.
			time.Sleep(delay)
			engine.AddRequest(request)
		}(engines[i], request, time.Millisecond*time.Duration(i*350))
	}

	// Run all engines
	runEngines(engines, workId, outCh, errCh, done, 0)
}

func TestEngineJobTimeout(t *testing.T) {
	log.Verbose("Running test with tss works starting at different time.")
	n := 4

	privKeys, nodes, pIDs, savedData := getEngineTestData(n)

	errCh := make(chan error)
	outCh := make(chan *p2pDataWrapper)
	engines := make([]Engine, n)
	workId := "presign0"
	done := make(chan bool)
	finishedWorkerCount := 0
	outputLock := &sync.Mutex{}
	pidString := ""
	presignIds := make([]string, n)
	pidStrings := make([]string, n)
	for i := range presignIds {
		presignIds[i] = fmt.Sprintf("%s-%d", workId, i)
		pidString = pidString + pIDs[i].Id
		if i < n-1 {
			pidString = pidString + ","
		}
	}
	for i := range presignIds {
		pidStrings[i] = pidString
	}

	for i := 0; i < n; i++ {
		config := NewDefaultEngineConfig()
		config.KeygenJobTimeout = time.Second

		engines[i] = NewEngine(
			nodes[i],
			NewMockConnectionManager(nodes[i].PeerId.String(), outCh),
			getMokDbForAvailManager(presignIds, pidStrings),
			&helper.MockEngineCallback{
				OnWorkFailedFunc: func(request *types.WorkRequest, culprits []*tss.PartyID) {
					outputLock.Lock()
					defer outputLock.Unlock()

					require.NotEmpty(t, culprits)
					finishedWorkerCount += 1
					if finishedWorkerCount == n {
						done <- true
					}
				},
			},
			privKeys[i],
			config,
		)
		engines[i].AddNodes(nodes)
	}

	// Start all engines
	for i := 0; i < n; i++ {
		request := types.NewPresignRequest(workId, n, helper.CopySortedPartyIds(pIDs), savedData[i], true)

		go func(engine Engine, request *types.WorkRequest, delay time.Duration) {
			// Deplay starting each engine to simulate that different workers can start at different times.
			time.Sleep(delay)
			engine.AddRequest(request)
		}(engines[i], request, time.Millisecond*time.Duration(i*350))
	}

	// Run all engines
	runEngines(engines, workId, outCh, errCh, done, 2*time.Second)
}

func runEngines(engines []Engine, workId string, outCh chan *p2pDataWrapper, errCh chan error, done chan bool, delay time.Duration) {
	// Run all engines
	for {
		select {
		case err := <-errCh:
			panic(err)
		case <-done:
			return
		case <-time.After(time.Second * 60):
			panic("Test timeout")

		case p2pMsgWrapper := <-outCh:
			for _, engine := range engines {
				defaultEngine := engine.(*DefaultEngine)
				if defaultEngine.myNode.PeerId.String() == p2pMsgWrapper.To {
					signedMessage := &common.SignedMessage{}
					if err := json.Unmarshal(p2pMsgWrapper.msg.Data, signedMessage); err != nil {
						panic(err)
					}

					time.Sleep(delay)
					if err := engine.ProcessNewMessage(signedMessage.TssMessage); err != nil {
						panic(err)
					}
					break
				}
			}
		}
	}
}
