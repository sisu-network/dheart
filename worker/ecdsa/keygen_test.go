package ecdsa

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	"github.com/sisu-network/dheart/types/common"
	"github.com/sisu-network/dheart/worker"
	"github.com/sisu-network/tss-lib/ecdsa/keygen"
	"github.com/stretchr/testify/assert"
)

//--- Miscellaneous helpers functions -- /

func TestKeygenEndToEnd(t *testing.T) {
	totalParticipants := 15
	threshold := 1
	batchSize := 1

	pIDs := generatePartyIds(totalParticipants)
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
		preparams := loadPreparams(i)

		workers[i] = NewKeygenWorker(
			fmt.Sprintf("worker%d", i),
			batchSize,
			pIDs,
			pIDs[i],
			preparams,
			threshold,
			NewTestDispatcher(outCh),
			errCh,
			NewTestKeygenCallback(cb),
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
	// saveKeysignOutput(finalOutput)
}

func generateTestPreparams(n int) {
	for i := 0; i < n; i++ {
		preParams, _ := keygen.GeneratePreParams(1 * time.Minute)
		bz, err := json.Marshal(preParams)
		if err != nil {
			panic(err)
		}

		err = saveTestPreparams(i, bz)
		if err != nil {
			panic(err)
		}
	}
}

func saveTestPreparams(index int, bz []byte) error {
	fileName := getTestSavedFileName(testPreparamsFixtureDirFormat, testPreparamsFixtureFileFormat, index)
	return ioutil.WriteFile(fileName, bz, 0644)
}

func loadPreparams(index int) *keygen.LocalPreParams {
	fileName := getTestSavedFileName(testPreparamsFixtureDirFormat, testPreparamsFixtureFileFormat, index)
	bz, err := ioutil.ReadFile(fileName)
	if err != nil {
		panic(err)
	}

	preparams := &keygen.LocalPreParams{}
	err = json.Unmarshal(bz, preparams)
	if err != nil {
		panic(err)
	}

	return preparams
}

func saveKeysignOutput(outputs []*keygen.LocalPartySaveData) error {
	for i, output := range outputs {
		fileName := getTestSavedFileName(testKeygenSavedDataFixtureDirFormat, testKeygenSavedDataFixtureFileFormat, i)

		bz, err := json.Marshal(output)
		if err != nil {
			panic(err)
		}

		if err := ioutil.WriteFile(fileName, bz, 0644); err != nil {
			return err
		}
	}

	return nil
}
