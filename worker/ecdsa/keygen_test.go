package ecdsa

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/sisu-network/dheart/types/common"
	"github.com/sisu-network/dheart/worker"
	"github.com/sisu-network/dheart/worker/helper"
	"github.com/sisu-network/dheart/worker/types"
	"github.com/sisu-network/tss-lib/ecdsa/keygen"
	"github.com/stretchr/testify/assert"
)

//--- Miscellaneous helpers functions -- /

func TestKeygenEndToEnd(t *testing.T) {
	totalParticipants := 6
	threshold := 1
	batchSize := 1

	pIDs := helper.GetTestPartyIds(totalParticipants)

	errCh := make(chan error)
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
			WorkType:    types.ECDSA_KEYGEN,
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
			helper.NewTestDispatcher(outCh),
			helper.NewMockDatabase(),
			errCh,
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
		)
	}

	// Start all workers
	startAllWorkers(workers)

	// Run all workers
	runAllWorkers(workers, outCh, errCh, done)

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
	helper.SaveKeygenOutput(finalOutput)
}

func generateTestPreparams(n int) {
	for i := 0; i < n; i++ {
		preParams, _ := keygen.GeneratePreParams(1 * time.Minute)
		bz, err := json.Marshal(preParams)
		if err != nil {
			panic(err)
		}

		err = helper.SaveTestPreparams(i, bz)
		if err != nil {
			panic(err)
		}
	}
}
