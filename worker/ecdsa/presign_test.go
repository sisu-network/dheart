package ecdsa

import (
	"math/big"
	"sync"
	"testing"

	"github.com/sisu-network/dheart/types/common"
	"github.com/sisu-network/dheart/worker"
	"github.com/sisu-network/dheart/worker/helper"
	"github.com/sisu-network/dheart/worker/types"
	"github.com/sisu-network/tss-lib/ecdsa/presign"
	"github.com/sisu-network/tss-lib/tss"
	"github.com/stretchr/testify/assert"
)

// func TestPresignBigTest(t *testing.T) {
// 	fmt.Printf("START: ACTIVE GOROUTINES: %d\n", runtime.NumGoroutine())
// 	for i := 0; i < 3; i++ {
// 		TestPresignEndToEnd(t)
// 	}

// 	fmt.Printf("END: ACTIVE GOROUTINES: %d\n", runtime.NumGoroutine())
// }

func TestPresignEndToEnd(t *testing.T) {
	n := 4
	batchSize := 1

	pIDs := helper.GetTestPartyIds(n)

	presignInputs := helper.LoadKeygenSavedData(pIDs)
	outCh := make(chan *common.TssMessage)
	errCh := make(chan error)
	workers := make([]worker.Worker, n)
	done := make(chan bool)
	finishedWorkerCount := 0

	presignOutputs := make([][]*presign.LocalPresignData, len(pIDs)) // n * batchSize
	outputLock := &sync.Mutex{}

	for i := 0; i < n; i++ {
		request := &types.WorkRequest{
			WorkId:       "Presign0",
			WorkType:     types.ECDSA_PRESIGN,
			AllParties:   helper.CopySortedPartyIds(pIDs),
			PresignInput: presignInputs[i],
			Threshold:    len(pIDs) - 1,
			N:            n,
		}

		workerIndex := i

		worker := NewPresignWorker(
			batchSize,
			request,
			pIDs[i],
			helper.NewTestDispatcher(outCh),
			helper.NewMockDatabase(),
			errCh,
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
	helper.SavePresignData(n, presignOutputs, 0)
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
