package ecdsa

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/sisu-network/dheart/types/common"
	"github.com/sisu-network/dheart/worker"
	"github.com/sisu-network/dheart/worker/helper"
	"github.com/sisu-network/tss-lib/ecdsa/keygen"
	"github.com/stretchr/testify/assert"
)

//--- Miscellaneous helpers functions -- /

func TestKeygenEndToEnd(t *testing.T) {
	totalParticipants := 15
	threshold := 1
	batchSize := 1

	pIDs := helper.GetTestPartyIds(totalParticipants)

	errCh := make(chan error)
	outCh := make(chan *common.TssMessage)

	done := make(chan bool)
	workers := make([]worker.Worker, totalParticipants)
	finishedWorkerCount := 0

	finalOutput := make([][]*keygen.LocalPartySaveData, len(pIDs)) // n * batchSize
	cb := func(workerId string, data []*keygen.LocalPartySaveData) {
		for i, worker := range workers {
			if worker.GetPartyId() == workerId {
				finalOutput[i] = data
				break
			}
		}

		finishedWorkerCount += 1

		if finishedWorkerCount == totalParticipants {
			done <- true
		}
	}

	// Generates n workers
	for i := 0; i < totalParticipants; i++ {
		preparams := helper.LoadPreparams(i)

		workers[i] = NewKeygenWorker(
			"Keygen0",
			batchSize,
			pIDs,
			pIDs[i],
			preparams,
			threshold,
			helper.NewTestDispatcher(outCh),
			errCh,
			helper.NewTestKeygenCallback(cb),
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
	// helper.SaveKeygenOutput(finalOutput)
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
