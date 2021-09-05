package core

import (
	"encoding/json"
	"math/big"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/sisu-network/dheart/types/common"
	"github.com/sisu-network/dheart/utils"
	"github.com/sisu-network/dheart/worker/helper"
	"github.com/sisu-network/dheart/worker/types"
	"github.com/sisu-network/tss-lib/ecdsa/presign"
	"github.com/sisu-network/tss-lib/tss"
	"github.com/stretchr/testify/assert"
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
	outputLock := &sync.Mutex{}

	cb := func(workerIndex int, workerId string, data []*presign.LocalPresignData) {
		outputLock.Lock()
		defer outputLock.Unlock()

		finishedWorkerCount += 1
		if finishedWorkerCount == n {
			done <- true
		}
	}

	for i := 0; i < n; i++ {
		engines[i] = NewEngine(nodes[i], NewMockConnectionManager(nodes[i].PeerId.String(), outCh),
			helper.NewMockDatabase(), helper.NewMockEnginePresignCallback(i, cb), privKeys[i])
		engines[i].AddNodes(nodes)
	}

	// Start all engines
	for i := 0; i < n; i++ {
		request := types.NewPresignRequest(workId, n, helper.CopySortedPartyIds(pIDs), *savedData[i], true)

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
		case <-time.After(time.Second * 60):
			panic("Test timeout")

		case p2pMsgWrapper := <-outCh:
			for _, engine := range engines {
				if engine.myNode.PeerId.String() == p2pMsgWrapper.To {
					signedMessage := &common.SignedMessage{}
					if err := json.Unmarshal(p2pMsgWrapper.msg.Data, signedMessage); err != nil {
						panic(err)
					}

					engine.ProcessNewMessage(signedMessage.TssMessage)
					break
				}
			}
		}
	}
}

// TestGetPresignData tests selecting a set of presigns in the database that matches available
// party ids.
func TestGetPresignData(t *testing.T) {
	expectedWorkId := []string{"testwork2", "testwork3"}

	presignPids := []string{"1,2,4", "2,3,5", "2,3,5", "3,4,6"}
	workIds := make([]string, len(presignPids))
	batchIndexes := make([]int, len(presignPids))

	for i := range presignPids {
		workIds[i] = "testwork" + strconv.Itoa(i)
		batchIndexes[i] = 0
	}

	// Mock get all presigns functions.
	mockDb := &helper.MockDatabase{
		GetAvailablePresignShortFormFunc: func() ([]string, []string, []int, error) {
			return presignPids, workIds, batchIndexes, nil
		},

		LoadPresignFunc: func(workId string, batchIndexes []int) ([]*presign.LocalPresignData, error) {
			count := 0
			for _, w := range workIds {
				if w == workId {
					count++
				}
			}

			ret := make([]*presign.LocalPresignData, count)
			return ret, nil
		},
	}

	// Create new engine
	privKeys, nodes, _, _ := getEngineTestData(1)
	engine := NewEngine(nodes[0], NewMockConnectionManager(nodes[0].PeerId.String(), nil),
		mockDb, helper.NewMockEnginePresignCallback(0, nil), privKeys[0])

	engine.Init()

	pids := []string{"2", "3", "4", "5", "6", "7"}
	partyIds := make([]*tss.PartyID, len(pids))
	for i := 0; i < len(pids); i++ {
		partyIds[i] = tss.NewPartyID(pids[i], "", big.NewInt(int64(i+1)))
	}

	// Runs presign selection.
	data, _ := engine.GetPresignData(len(expectedWorkId), 3, partyIds)
	assert.Equal(t, len(expectedWorkId), len(data))

	// After consuming some presign data, the selected presigns are removed from the available presign
	// pool.
	assert.Equal(t, len(presignPids)-len(expectedWorkId), len(engine.presignsManager.available))
}
