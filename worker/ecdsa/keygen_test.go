package ecdsa

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/sisu-network/dheart/types/common"
	"github.com/sisu-network/dheart/worker"
	"github.com/sisu-network/dheart/worker/helper"
	"github.com/sisu-network/dheart/worker/types"
	"github.com/sisu-network/tss-lib/ecdsa/keygen"
)

//--- Miscellaneous helpers functions -- /

func TestKeygenEndToEnd(t *testing.T) {
	totalParticipants := 6
	threshold := 1
	batchSize := 1

	pIDs := helper.GetTestPartyIds(totalParticipants)

	outCh := make(chan *common.TssMessage)

	done := make(chan bool)
	workers := make([]worker.Worker, totalParticipants)
	finishedWorkerCount := 0

	finalOutput := make([][]*keygen.LocalPartySaveData, len(pIDs)) // n * batchSize
	outputLock := &sync.Mutex{}

	// Generates n workers
	for i := 0; i < totalParticipants; i++ {
		preparams := helper.LoadPreparams(i)

		request := &types.WorkRequest{
			WorkId:      "Keygen0",
			WorkType:    types.EcdsaKeygen,
			AllParties:  helper.CopySortedPartyIds(pIDs),
			KeygenInput: preparams,
			Threshold:   threshold,
			N:           totalParticipants,
		}

		workerIndex := i

		workers[i] = NewKeygenWorker(
			batchSize,
			request,
			pIDs[i],
			helper.NewTestDispatcher(outCh, 0, 0),
			helper.NewMockDatabase(),
			&helper.MockWorkerCallback{
				OnWorkKeygenFinishedFunc: func(request *types.WorkRequest, data []*keygen.LocalPartySaveData) {
					outputLock.Lock()
					defer outputLock.Unlock()

					finalOutput[workerIndex] = data
					finishedWorkerCount += 1

					if finishedWorkerCount == totalParticipants {
						done <- true
					}
				},
			},
			10*time.Minute,
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

	// Save final outputs. Uncomment this line when you want to save keygen output to fixtures.
	assert.NoError(t, helper.SaveKeygenOutput(finalOutput))
}

func TestKeygenTimeout(t *testing.T) {
	totalParticipants := 6
	threshold := 1
	batchSize := 1

	pIDs := helper.GetTestPartyIds(totalParticipants)

	outCh := make(chan *common.TssMessage)

	done := make(chan bool)
	workers := make([]worker.Worker, totalParticipants)
	outputLock := &sync.Mutex{}
	failedWorkCounts := 0

	// Generates n workers
	for i := 0; i < totalParticipants; i++ {
		preparams := helper.LoadPreparams(i)

		request := &types.WorkRequest{
			WorkId:      "Keygen0",
			WorkType:    types.EcdsaKeygen,
			AllParties:  helper.CopySortedPartyIds(pIDs),
			KeygenInput: preparams,
			Threshold:   threshold,
			N:           totalParticipants,
		}

		workers[i] = NewKeygenWorker(
			batchSize,
			request,
			pIDs[i],
			helper.NewTestDispatcher(outCh, 0, 2*time.Second),
			helper.NewMockDatabase(),
			&helper.MockWorkerCallback{
				OnWorkFailedFunc: func(request *types.WorkRequest) {
					outputLock.Lock()
					defer outputLock.Unlock()

					failedWorkCounts++
					if failedWorkCounts == totalParticipants {
						done <- true
					}
				},
			},
			time.Second,
		)
	}

	// Start all workers
	startAllWorkers(workers)

	// Run all workers
	runAllWorkers(workers, outCh, done)
}
