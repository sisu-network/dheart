package ecdsa

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"runtime"
	"testing"

	"github.com/sisu-network/dheart/types/common"
	"github.com/sisu-network/dheart/worker"
	"github.com/sisu-network/tss-lib/ecdsa/keygen"
	"github.com/sisu-network/tss-lib/ecdsa/presign"
	"github.com/sisu-network/tss-lib/tss"
	"github.com/stretchr/testify/assert"
)

func TestPresignBigTest(t *testing.T) {
	fmt.Printf("START: ACTIVE GOROUTINES: %d\n", runtime.NumGoroutine())
	for i := 0; i < 3; i++ {
		TestPresignEndToEnd(t)
	}

	fmt.Printf("END: ACTIVE GOROUTINES: %d\n", runtime.NumGoroutine())
}

func TestPresignEndToEnd(t *testing.T) {
	n := 3
	batchSize := 4

	pIDs := generatePartyIds(n)

	savedData := loadKeygenSavedData(n)
	p2pCtx := tss.NewPeerContext(pIDs)
	outCh := make(chan *common.TssMessage)
	errCh := make(chan error)
	workers := make([]worker.Worker, n)
	done := make(chan bool)
	finishedWorkerCount := 0

	presignOutputs := make([][]*presign.LocalPresignData, len(pIDs)) // n * batchSize
	cb := func(workerId string, data []*presign.LocalPresignData) {
		for i, worker := range workers {
			if worker.GetPartyId() == workerId {
				presignOutputs[i] = data
				break
			}
		}

		finishedWorkerCount += 1

		if finishedWorkerCount == n {
			done <- true
		}
	}

	for i := 0; i < n; i++ {
		params := tss.NewParameters(p2pCtx, pIDs[i], len(pIDs), n-1)
		worker := NewPresignWorker(
			fmt.Sprintf("worker%d", i),
			batchSize,
			pIDs,
			pIDs[i],
			params,
			savedData[i],
			NewTestDispatcher(outCh),
			errCh,
			NewTestPresignCallback(cb),
		)

		workers[i] = worker
	}

	// Start all workers
	startAllWorkers(workers)

	// Run all workers
	runAllWorkers(workers, outCh, errCh, done)

	verifyPubKey(t, n, batchSize, presignOutputs)

	// Save presign data. Uncomment this line to save presign data fixtures after test (these
	// fixtures could be used in signing test)
	// savePresignData(n, presignOutputs, 0)
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

func loadKeygenSavedData(n int) []*keygen.LocalPartySaveData {
	savedData := make([]*keygen.LocalPartySaveData, n)

	for i := 0; i < n; i++ {
		fileName := getTestSavedFileName(testKeygenSavedDataFixtureDirFormat, testKeygenSavedDataFixtureFileFormat, i)

		bz, err := ioutil.ReadFile(fileName)
		if err != nil {
			panic(err)
		}

		data := &keygen.LocalPartySaveData{}
		if err := json.Unmarshal(bz, data); err != nil {
			panic(err)
		}

		savedData[i] = data
	}

	return savedData
}

func savePresignData(n int, data [][]*presign.LocalPresignData, testIndex int) error {
	wrapper := &PresignDataWrapper{
		Outputs: data,
	}

	fileName := getTestSavedFileName(testPresignSavedDataFixtureDirFormat, testPresignSavedDataFixtureFileFormat, testIndex)

	bz, err := json.Marshal(wrapper)
	if err != nil {
		panic(err)
	}

	if err := ioutil.WriteFile(fileName, bz, 0644); err != nil {
		return err
	}

	return nil
}
