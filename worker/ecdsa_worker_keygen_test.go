package worker

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/sisu-network/dheart/core/config"
	"github.com/sisu-network/dheart/db"
	"github.com/sisu-network/dheart/types/common"
	"github.com/sisu-network/dheart/worker/types"
	"github.com/sisu-network/lib/log"
	"github.com/sisu-network/tss-lib/ecdsa/keygen"
)

//--- Miscellaneous helpers functions -- /

func TestEcWorkerKeygen_EndToEnd(t *testing.T) {
	totalParticipants := 6
	threshold := 1
	batchSize := 1

	pIDs := GetTestPartyIds(totalParticipants)

	outCh := make(chan *common.TssMessage)

	done := make(chan bool)
	workers := make([]Worker, totalParticipants)
	finishedWorkerCount := 0

	finalOutput := make([][]*keygen.LocalPartySaveData, len(pIDs)) // n * batchSize
	outputLock := &sync.Mutex{}

	// Generates n workers
	for i := 0; i < totalParticipants; i++ {
		preparams := LoadEcPreparams(i)

		request := &types.WorkRequest{
			WorkId:        "Keygen0",
			WorkType:      types.EcKeygen,
			AllParties:    CopySortedPartyIds(pIDs),
			BatchSize:     batchSize,
			EcKeygenInput: preparams,
			Threshold:     threshold,
			N:             totalParticipants,
		}

		workerIndex := i
		timeoutConfig := config.NewDefaultTimeoutConfig()
		timeoutConfig.MonitorMessageTimeout = time.Second * 60 // 1 minute

		workers[i] = NewKeygenWorker(
			request,
			pIDs[i],
			NewTestDispatcher(outCh, 0, 0),
			db.NewMockDatabase(),
			&MockWorkerCallback{
				OnWorkerResultFunc: func(request *types.WorkRequest, result *WorkerResult) {
					outputLock.Lock()
					defer outputLock.Unlock()

					finalOutput[workerIndex] = GetEcKeygenOutputs(result.JobResults)
					finishedWorkerCount += 1

					if finishedWorkerCount == totalParticipants {
						done <- true
					}
				},
			},
			timeoutConfig,
		)
	}

	// Start all workers
	startAllWorkers(workers)

	// Run all workers
	runAllWorkers(workers, outCh, done)

	// All outputs should have the same batch size.
	for i := 0; i < totalParticipants; i++ {
		assert.Equal(t, len(finalOutput[i]), batchSize)
	}

	for j := 0; j < batchSize; j++ {
		// Check that everyone has the same public key.
		for i := 0; i < totalParticipants; i++ {
			assert.Equal(t, finalOutput[i][j].ECDSAPub.X(), finalOutput[0][j].ECDSAPub.X())
			assert.Equal(t, finalOutput[i][j].ECDSAPub.Y(), finalOutput[0][j].ECDSAPub.Y())
		}
	}
}

func TestEcWorkerKeygen_Timeout(t *testing.T) {
	log.Verbose("Running TestKeygenTimeout test")
	totalParticipants := 6
	threshold := 1
	batchSize := 1

	pIDs := GetTestPartyIds(totalParticipants)

	outCh := make(chan *common.TssMessage)

	done := make(chan bool)
	workers := make([]Worker, totalParticipants)
	outputLock := &sync.Mutex{}
	failedWorkCounts := 0

	// Generates n workers
	for i := 0; i < totalParticipants; i++ {
		preparams := LoadEcPreparams(i)

		request := &types.WorkRequest{
			WorkId:        "Keygen0",
			WorkType:      types.EcKeygen,
			AllParties:    CopySortedPartyIds(pIDs),
			BatchSize:     batchSize,
			EcKeygenInput: preparams,
			Threshold:     threshold,
			N:             totalParticipants,
		}

		cfg := config.NewDefaultTimeoutConfig()
		cfg.KeygenJobTimeout = time.Second
		cfg.MonitorMessageTimeout = time.Second * 60

		workers[i] = NewKeygenWorker(
			request,
			pIDs[i],
			NewTestDispatcher(outCh, 0, 2*time.Second),
			db.NewMockDatabase(),
			&MockWorkerCallback{
				OnWorkFailedFunc: func(request *types.WorkRequest) {
					outputLock.Lock()
					defer outputLock.Unlock()

					failedWorkCounts++
					if failedWorkCounts == totalParticipants {
						done <- true
					}
				},
			},
			cfg,
		)
	}

	// Start all workers
	startAllWorkers(workers)

	// Run all workers
	runAllWorkers(workers, outCh, done)
}

// Dont' delete this. This is used for generating preparams for tests.
// We do not use that right now because we have generate the preparams and save them into a file.
// In the future, if we want to re-generate the preparams, we will need to call this function.
func generateTestPreparams(n int) {
	for i := 0; i < n; i++ {
		preParams, _ := keygen.GeneratePreParams(1 * time.Minute)
		bz, err := json.Marshal(preParams)
		if err != nil {
			panic(err)
		}

		err = SaveTestPreparams(i, bz)
		if err != nil {
			panic(err)
		}
	}
}
