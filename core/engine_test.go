package core

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/sisu-network/dheart/core/components"
	"github.com/sisu-network/dheart/core/config"
	htypes "github.com/sisu-network/dheart/types"
	"github.com/sisu-network/dheart/types/common"
	"github.com/sisu-network/dheart/worker"
	"github.com/sisu-network/dheart/worker/types"
	"github.com/sisu-network/lib/log"
	"github.com/sisu-network/tss-lib/tss"
)

func runEnginesWithDelay(engines []Engine, workId string, outCh chan *p2pDataWrapper, errCh chan error, done chan bool, delay time.Duration) {
	runEngineWithOptions(engines, workId, outCh, errCh, done, delay, nil, false)
}

func getDropMsgPair(from, to string) string {
	return fmt.Sprintf("%s__%s", from, to)
}

func runEnginesWithDroppedMessages(engines []Engine, workId string, outCh chan *p2pDataWrapper,
	errCh chan error, done chan bool, drop map[string]map[string]bool) {
	runEngineWithOptions(engines, workId, outCh, errCh, done, time.Second*0, drop, false)
}

func runEnginesWithDuplicatedMessage(engines []Engine, workId string, outCh chan *p2pDataWrapper,
	errCh chan error, done chan bool) {
	runEngineWithOptions(engines, workId, outCh, errCh, done, time.Second*0, nil, true)
}

// Run an engine with possible message drop. The message drop is defined in the drop map. Each
// message from -> to with a specific type will be dropped once and removed from the map after
// it is dropped.
func runEngineWithOptions(engines []Engine, workId string, outCh chan *p2pDataWrapper,
	errCh chan error, done chan bool, delay time.Duration, drop map[string]map[string]bool, duplicateMessage bool) {
	lock := &sync.RWMutex{}

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
				defaultEngine := engine.(*defaultEngine)
				if defaultEngine.myNode.PeerId.String() == p2pMsgWrapper.To {
					signedMessage := &common.SignedMessage{}
					if err := json.Unmarshal(p2pMsgWrapper.msg.Data, signedMessage); err != nil {
						panic(err)
					}

					// Check if the message should be dropped
					if drop != nil && signedMessage.TssMessage.Type == common.TssMessage_UPDATE_MESSAGES {
						msg := signedMessage.TssMessage

						shouldDrop := false
						lock.RLock()
						pair := getDropMsgPair(msg.From, defaultEngine.myPid.Id)
						dropMsgs := drop[pair]
						if dropMsgs != nil && dropMsgs[msg.UpdateMessages[0].Round] {
							// This message needs to be drop
							log.Verbose("Droping message: ", pair, msg.UpdateMessages[0].Round)
							shouldDrop = true
						}
						lock.RUnlock()

						if shouldDrop {
							lock.Lock()
							dropMsgs[msg.UpdateMessages[0].Round] = false
							lock.Unlock()

							break
						}
					}

					time.Sleep(delay)

					if err := engine.ProcessNewMessage(signedMessage.TssMessage); err != nil {
						panic(err)
					}
					if duplicateMessage {
						// Run this again.
						if err := engine.ProcessNewMessage(signedMessage.TssMessage); err != nil {
							panic(err)
						}
					}
					break
				}
			}
		}
	}
}

func getEngineTestPresignAndPids(n int, workId string, pIDs tss.SortedPartyIDs) ([]string, []string) {
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

	return presignIds, pidStrings
}

func TestEngine_DelayStart(t *testing.T) {
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

	presignIds, pidStrings := getEngineTestPresignAndPids(n, workId, pIDs)

	for i := 0; i < n; i++ {
		engines[i] = NewEngine(
			nodes[i],
			NewMockConnectionManager(nodes[i].PeerId.String(), outCh),
			components.GetMokDbForAvailManager(presignIds, pidStrings),
			&MockEngineCallback{
				OnWorkSigningFinishedFunc: func(request *types.WorkRequest, result *htypes.KeysignResult) {
					outputLock.Lock()
					defer outputLock.Unlock()

					finishedWorkerCount += 1
					if finishedWorkerCount == n {
						done <- true
					}
				},
			},
			privKeys[i],
			config.NewDefaultTimeoutConfig(),
		)
		engines[i].AddNodes(nodes)
	}

	// Start all engines
	for i := 0; i < n; i++ {
		request := types.NewEcSigningRequest(workId, worker.CopySortedPartyIds(pIDs), n-1,
			make([][]byte, 1), []string{"ganache1"}, savedData[i])

		go func(engine Engine, request *types.WorkRequest, delay time.Duration) {
			// Deplay starting each engine to simulate that different workers can start at different times.
			time.Sleep(delay)
			engine.AddRequest(request)
		}(engines[i], request, time.Millisecond*time.Duration(i*350))
	}

	// Run all engines
	runEnginesWithDelay(engines, workId, outCh, errCh, done, 0)
}

func TestEngine_SendDuplicateMessage(t *testing.T) {
	t.Parallel()

	n := 4

	privKeys, nodes, pIDs, savedData := getEngineTestData(n)

	errCh := make(chan error)
	outCh := make(chan *p2pDataWrapper)
	engines := make([]Engine, n)
	workId := "presign0"
	done := make(chan bool)
	finishedWorkerCount := 0
	outputLock := &sync.Mutex{}

	presignIds, pidStrings := getEngineTestPresignAndPids(n, workId, pIDs)

	for i := 0; i < n; i++ {
		engines[i] = NewEngine(
			nodes[i],
			NewMockConnectionManager(nodes[i].PeerId.String(), outCh),
			components.GetMokDbForAvailManager(presignIds, pidStrings),
			&MockEngineCallback{
				OnWorkSigningFinishedFunc: func(request *types.WorkRequest, result *htypes.KeysignResult) {
					outputLock.Lock()
					defer outputLock.Unlock()

					finishedWorkerCount += 1
					if finishedWorkerCount == n {
						done <- true
					}
				},
			},
			privKeys[i],
			config.NewDefaultTimeoutConfig(),
		)
		engines[i].AddNodes(nodes)
	}

	// Start all engines
	for i := 0; i < n; i++ {
		request := types.NewEcSigningRequest(workId, worker.CopySortedPartyIds(pIDs), n-1,
			make([][]byte, 1), []string{"ganache1"}, savedData[i])

		go func(engine Engine, request *types.WorkRequest, delay time.Duration) {
			// Deplay starting each engine to simulate that different workers can start at different times.
			time.Sleep(delay)
			engine.AddRequest(request)
		}(engines[i], request, time.Millisecond*time.Duration(i*350))
	}

	runEnginesWithDuplicatedMessage(engines, workId, outCh, errCh, done)
}

func TestEngine_JobTimeout(t *testing.T) {
	n := 4

	privKeys, nodes, pIDs, savedData := getEngineTestData(n)

	errCh := make(chan error)
	outCh := make(chan *p2pDataWrapper)
	engines := make([]Engine, n)
	workId := "presign0"
	done := make(chan bool)
	finishedWorkerCount := 0
	outputLock := &sync.Mutex{}

	presignIds, pidStrings := getEngineTestPresignAndPids(n, workId, pIDs)

	for i := 0; i < n; i++ {
		config := config.NewDefaultTimeoutConfig()
		config.SelectionLeaderTimeout = time.Second * 1
		config.SelectionMemberTimeout = time.Second * 2
		config.PresignJobTimeout = time.Second * 3

		engines[i] = NewEngine(
			nodes[i],
			NewMockConnectionManager(nodes[i].PeerId.String(), outCh),
			components.GetMokDbForAvailManager(presignIds, pidStrings),
			&MockEngineCallback{
				OnWorkFailedFunc: func(request *types.WorkRequest, culprits []*tss.PartyID) {
					outputLock.Lock()
					defer outputLock.Unlock()

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
		request := types.NewEcSigningRequest(workId, worker.CopySortedPartyIds(pIDs), n-1,
			make([][]byte, 1), []string{"ganache1"}, savedData[i])

		go func(engine Engine, request *types.WorkRequest, delay time.Duration) {
			// Delay starting each engine to simulate that different workers can start at different times.
			time.Sleep(delay)
			engine.AddRequest(request)
		}(engines[i], request, time.Millisecond*time.Duration(i*350))
	}

	// Run all engines
	runEnginesWithDelay(engines, workId, outCh, errCh, done, 2*time.Second)
}

func TestEngine_MissingMessages(t *testing.T) {
	n := 4

	privKeys, nodes, pIDs, savedData := getEngineTestData(n)

	errCh := make(chan error)
	outCh := make(chan *p2pDataWrapper, n*10)
	engines := make([]Engine, n)
	workId := "presign0"
	done := make(chan bool)
	finishedWorkerCount := 0
	outputLock := &sync.Mutex{}

	presignIds, pidStrings := getEngineTestPresignAndPids(n, workId, pIDs)

	for i := 0; i < n; i++ {
		config := config.NewDefaultTimeoutConfig()
		config.MonitorMessageTimeout = time.Duration(time.Second * 1)

		engines[i] = NewEngine(
			nodes[i],
			NewMockConnectionManager(nodes[i].PeerId.String(), outCh),
			components.GetMokDbForAvailManager(presignIds, pidStrings),
			&MockEngineCallback{
				OnWorkSigningFinishedFunc: func(request *types.WorkRequest, result *htypes.KeysignResult) {
					outputLock.Lock()
					defer outputLock.Unlock()

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
		request := types.NewEcSigningRequest(workId, worker.CopySortedPartyIds(pIDs), n-1, make([][]byte, 1),
			[]string{"ganache"}, savedData[i])

		go func(engine Engine, request *types.WorkRequest) {
			engine.AddRequest(request)
		}(engines[i], request)
	}

	drop := make(map[string]map[string]bool)
	drop[getDropMsgPair(nodes[0].PartyId.Id, nodes[3].PartyId.Id)] = map[string]bool{
		"SignRound2Message": true,
		"SignRound3Message": true,
		"SignRound4Message": true,
	}

	drop[getDropMsgPair(nodes[1].PartyId.Id, nodes[3].PartyId.Id)] = map[string]bool{
		"SignRound2Message": true,
		"SignRound3Message": true,
		"SignRound4Message": true,
	}

	drop[getDropMsgPair(nodes[2].PartyId.Id, nodes[3].PartyId.Id)] = map[string]bool{
		"SignRound2Message": true,
		"SignRound3Message": true,
		"SignRound4Message": true,
	}

	drop[getDropMsgPair(nodes[1].PartyId.Id, nodes[2].PartyId.Id)] = map[string]bool{
		"SignRound4Message": true,
	}

	// Run all engines
	runEnginesWithDroppedMessages(engines, workId, outCh, errCh, done, drop)
}
