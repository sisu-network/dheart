package worker

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sisu-network/dheart/core/config"
	"github.com/sisu-network/dheart/db"
	"github.com/sisu-network/dheart/types/common"
	"github.com/sisu-network/dheart/worker/types"
	"github.com/sisu-network/lib/log"
	"github.com/sisu-network/tss-lib/ecdsa/presign"
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
	log.Verbose("Running TestPresign_EndToEnd")
	n := 4
	batchSize := 1
	threshold := n - 1

	pIDs := GetTestPartyIds(n)

	presignInputs := LoadEcKeygenSavedData(pIDs)
	outCh := make(chan *common.TssMessage)
	workers := make([]Worker, n)
	done := make(chan bool)
	finishedWorkerCount := 0

	presignOutputs := make([][]*presign.LocalPresignData, 0)
	outputLock := &sync.Mutex{}

	for i := 0; i < n; i++ {
		request := types.NewPresignRequest(
			"Presign0",
			CopySortedPartyIds(pIDs),
			threshold,
			presignInputs[i],
			false,
			batchSize,
		)

		cfg := config.NewDefaultTimeoutConfig()
		cfg.MonitorMessageTimeout = time.Second * 60

		worker := NewPresignWorker(
			request,
			pIDs[i],
			NewTestDispatcher(outCh, 0, 0),
			db.NewMockDatabase(),
			&MockWorkerCallback{
				OnWorkerResultFunc: func(request *types.WorkRequest, result *WorkerResult) {
					outputLock.Lock()
					defer outputLock.Unlock()

					presignOutputs = append(presignOutputs, result.EcPresignData)
					finishedWorkerCount += 1
					if finishedWorkerCount == n {
						done <- true
					}
				},

				OnNodeNotSelectedFunc: func(request *types.WorkRequest) {
					outputLock.Lock()
					defer outputLock.Unlock()

					finishedWorkerCount += 1
					if finishedWorkerCount == n {
						done <- true
					}
				},
			},
			cfg,
			1,
		)

		workers[i] = worker
	}

	// Start all workers
	startAllWorkers(workers)

	// Run all workers
	runAllWorkers(workers, outCh, done)

	// Do not delete
	// Save presign data. Uncomment this line to save presign data fixtures after test (these
	// fixtures could be used in signing test)
	// SavePresignData(n, presignOutputs, pIDs, 0)
}

func TestPresign_PreExecutionTimeout(t *testing.T) {
	log.Verbose("Running TestPresign_PreExecutionTimeout")
	n := 4
	batchSize := 1
	pIDs := GetTestPartyIds(n)
	presignInputs := LoadEcKeygenSavedData(pIDs)
	outCh := make(chan *common.TssMessage)
	workers := make([]Worker, n)
	done := make(chan bool)

	var numFailedWorkers uint32

	for i := 0; i < n; i++ {
		request := types.NewPresignRequest(
			"Presign0",
			CopySortedPartyIds(pIDs),
			len(pIDs)-1,
			presignInputs[i],
			false,
			batchSize,
		)

		cfg := config.NewDefaultTimeoutConfig()
		cfg.SelectionLeaderTimeout = time.Second * 2

		worker := NewPresignWorker(
			request,
			pIDs[i],
			NewTestDispatcher(outCh, cfg.SelectionLeaderTimeout+1*time.Second, 0),
			db.NewMockDatabase(),
			&MockWorkerCallback{
				OnWorkFailedFunc: func(request *types.WorkRequest) {
					if n := atomic.AddUint32(&numFailedWorkers, 1); n == 4 {
						done <- true
					}
				},
			},
			cfg,
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
	log.Verbose("Running TestPresign_ExecutionTimeout")
	n := 4
	batchSize := 1
	pIDs := GetTestPartyIds(n)
	presignInputs := LoadEcKeygenSavedData(pIDs)
	outCh := make(chan *common.TssMessage)
	workers := make([]Worker, n)
	done := make(chan bool)

	var numFailedWorkers uint32

	for i := 0; i < n; i++ {
		request := types.NewPresignRequest(
			"Presign0",
			CopySortedPartyIds(pIDs),
			len(pIDs)-1,
			presignInputs[i],
			false,
			batchSize,
		)

		cfg := config.NewDefaultTimeoutConfig()
		cfg.PresignJobTimeout = time.Second

		worker := NewPresignWorker(
			request,
			pIDs[i],
			NewTestDispatcher(outCh, 0, 2*time.Second),
			db.NewMockDatabase(),
			&MockWorkerCallback{
				OnWorkFailedFunc: func(request *types.WorkRequest) {
					if n := atomic.AddUint32(&numFailedWorkers, 1); n == 4 {
						done <- true
					}
				},
			},
			cfg,
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

// Runs test when we have a strict threshold < n - 1.
func TestPresign_Threshold(t *testing.T) {
	log.Verbose("Running TestPresign_Threshold")
	n := 4
	threshold := 2
	batchSize := 1

	pIDs := GetTestPartyIds(n)

	presignInputs := LoadEcKeygenSavedData(pIDs)
	outCh := make(chan *common.TssMessage)
	workers := make([]Worker, n)
	done := make(chan bool)
	finishedWorkerCount := 0

	presignOutputs := make([][]*presign.LocalPresignData, 0) // n * batchSize
	outputLock := &sync.Mutex{}

	for i := 0; i < n; i++ {
		request := types.NewPresignRequest(
			"Presign0",
			CopySortedPartyIds(pIDs),
			threshold,
			presignInputs[i],
			false,
			batchSize,
		)

		cfg := config.NewDefaultTimeoutConfig()
		cfg.MonitorMessageTimeout = time.Second * 60

		worker := NewPresignWorker(
			request,
			pIDs[i],
			NewTestDispatcher(outCh, 0, 0),
			db.NewMockDatabase(),
			&MockWorkerCallback{
				OnWorkerResultFunc: func(request *types.WorkRequest, result *WorkerResult) {
					outputLock.Lock()
					defer outputLock.Unlock()

					presignOutputs = append(presignOutputs, result.EcPresignData)
					finishedWorkerCount += 1
					if finishedWorkerCount == n {
						done <- true
					}
				},

				OnNodeNotSelectedFunc: func(request *types.WorkRequest) {
					outputLock.Lock()
					defer outputLock.Unlock()

					finishedWorkerCount += 1
					if finishedWorkerCount == n {
						done <- true
					}
				},
			},
			cfg,
			1,
		)

		workers[i] = worker
	}

	// Start all workers
	startAllWorkers(workers)

	// Run all workers
	runAllWorkers(workers, outCh, done)

	assert.Equal(t, threshold+1, len(presignOutputs), "Presign output length is not correct")

	// SavePresignData(n, presignOutputs, 2)
}
