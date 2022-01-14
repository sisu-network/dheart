package ecdsa

import (
	"math/big"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sisu-network/dheart/types/common"
	"github.com/sisu-network/dheart/worker"
	"github.com/sisu-network/dheart/worker/helper"
	"github.com/sisu-network/dheart/worker/types"
	"github.com/sisu-network/tss-lib/ecdsa/presign"
	"github.com/sisu-network/tss-lib/tss"
	"github.com/stretchr/testify/assert"
)

// Do not remove
// func TestPresignBigTest(t *testing.T) {
// 	fmt.Printf("START: ACTIVE GOROUTINES: %d\n", runtime.NumGoroutine())
// 	for i := 0; i < 3; i++ {
// 		TestPresignEndToEnd(t)
// 	}

// 	fmt.Printf("END: ACTIVE GOROUTINES: %d\n", runtime.NumGoroutine())
// }

func TestPresign_EndToEnd(t *testing.T) {
	n := 4
	batchSize := 1

	pIDs := helper.GetTestPartyIds(n)

	presignInputs := helper.LoadKeygenSavedData(pIDs)
	outCh := make(chan *common.TssMessage)
	workers := make([]worker.Worker, n)
	done := make(chan bool)
	finishedWorkerCount := 0

	presignOutputs := make([][]*presign.LocalPresignData, len(pIDs)) // n * batchSize
	outputLock := &sync.Mutex{}

	for i := 0; i < n; i++ {
		request := types.NewPresignRequest(
			"Presign0",
			helper.CopySortedPartyIds(pIDs),
			len(pIDs)-1,
			presignInputs[i],
			false,
			batchSize,
		)

		workerIndex := i

		worker := NewPresignWorker(
			request,
			pIDs[i],
			helper.NewTestDispatcher(outCh, 0, 0),
			helper.NewMockDatabase(),
			&helper.MockWorkerCallback{
				OnWorkPresignFinishedFunc: func(request *types.WorkRequest, pids []*tss.PartyID, data []*presign.LocalPresignData) {
					outputLock.Lock()
					defer outputLock.Unlock()

					presignOutputs[workerIndex] = data
					finishedWorkerCount += 1
					if finishedWorkerCount == n {
						done <- true
					}
				},
			},
			10*time.Minute,
			1,
		)

		workers[i] = worker
	}

	// Start all workers
	startAllWorkers(workers)

	// Run all workers
	runAllWorkers(workers, outCh, done)

	verifyPubKey(t, n, batchSize, presignOutputs)

	// Do not delete
	// Save presign data. Uncomment this line to save presign data fixtures after test (these
	// fixtures could be used in signing test)
	// helper.SavePresignData(n, presignOutputs, 0)
}

func TestPresign_PreExecutionTimeout(t *testing.T) {
	n := 4
	batchSize := 1
	pIDs := helper.GetTestPartyIds(n)
	presignInputs := helper.LoadKeygenSavedData(pIDs)
	outCh := make(chan *common.TssMessage)
	workers := make([]worker.Worker, n)
	done := make(chan bool)

	var numFailedWorkers uint32

	for i := 0; i < n; i++ {
		request := types.NewPresignRequest(
			"Presign0",
			helper.CopySortedPartyIds(pIDs),
			len(pIDs)-1,
			presignInputs[i],
			false,
			batchSize,
		)

		worker := NewPresignWorker(
			request,
			pIDs[i],
			helper.NewTestDispatcher(outCh, PreExecutionRequestWaitTime+1*time.Second, 0),
			helper.NewMockDatabase(),
			&helper.MockWorkerCallback{
				OnWorkFailedFunc: func(request *types.WorkRequest) {
					if n := atomic.AddUint32(&numFailedWorkers, 1); n == 4 {
						done <- true
					}
				},
			},
			10*time.Minute,
			1,
		)

		workers[i] = worker
	}

	// Start all workers
	startAllWorkers(workers)

	// Run all workers
	runAllWorkers(workers, outCh, done)

	assert.EqualValues(t, 4, numFailedWorkers)
}

func TestPresign_ExecutionTimeout(t *testing.T) {
	n := 4
	batchSize := 1
	pIDs := helper.GetTestPartyIds(n)
	presignInputs := helper.LoadKeygenSavedData(pIDs)
	outCh := make(chan *common.TssMessage)
	workers := make([]worker.Worker, n)
	done := make(chan bool)

	var numFailedWorkers uint32

	for i := 0; i < n; i++ {
		request := types.NewPresignRequest(
			"Presign0",
			helper.CopySortedPartyIds(pIDs),
			len(pIDs)-1,
			presignInputs[i],
			false,
			batchSize,
		)

		worker := NewPresignWorker(
			request,
			pIDs[i],
			helper.NewTestDispatcher(outCh, 0, 2*time.Second),
			helper.NewMockDatabase(),
			&helper.MockWorkerCallback{
				OnWorkFailedFunc: func(request *types.WorkRequest) {
					if n := atomic.AddUint32(&numFailedWorkers, 1); n == 4 {
						done <- true
					}
				},
			},
			time.Second,
			1,
		)

		workers[i] = worker
	}

	// Start all workers
	startAllWorkers(workers)

	// Run all workers
	runAllWorkers(workers, outCh, done)

	assert.EqualValues(t, 4, numFailedWorkers)
}

func verifyPubKey(t *testing.T, n, batchSize int, presignOutputs [][]*presign.LocalPresignData) {
	for j := 0; j < batchSize; j++ {
		w := big.NewInt(0)
		for i := 0; i < n; i++ {
			w.Add(w, presignOutputs[i][j].W)
		}
		w.Mod(w, tss.EC().Params().N)

		px, py := tss.EC().ScalarBaseMult(w.Bytes())
		assert.Equal(t, px, presignOutputs[0][j].ECDSAPub.X())
		assert.Equal(t, py, presignOutputs[0][j].ECDSAPub.Y())
	}
}
