package ecdsa

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sisu-network/dheart/db"
	"github.com/sisu-network/dheart/types/common"
	"github.com/sisu-network/dheart/worker"
	"github.com/sisu-network/dheart/worker/helper"
	"github.com/sisu-network/dheart/worker/types"
	libCommon "github.com/sisu-network/tss-lib/common"
	"github.com/sisu-network/tss-lib/ecdsa/presign"
	"github.com/sisu-network/tss-lib/tss"
	"github.com/stretchr/testify/assert"
)

func mockDbForSigning(pids []*tss.PartyID, WorkId string, batchSize int) db.Database {
	pidString := ""
	for i, pid := range pids {
		pidString = pidString + pid.Id
		if i < len(pids)-1 {
			pidString = pidString + ","
		}
	}

	pidStrings := make([]string, batchSize)
	presignIds := make([]string, batchSize)
	for i := range presignIds {
		presignIds[i] = fmt.Sprintf("%s-%d", WorkId, i)
		pidStrings[i] = pidString
	}

	return &helper.MockDatabase{
		GetAvailablePresignShortFormFunc: func() ([]string, []string, error) {
			return presignIds, pidStrings, nil
		},

		LoadPresignFunc: func(presignIds []string) ([]*presign.LocalPresignData, error) {
			return make([]*presign.LocalPresignData, len(presignIds)), nil
		},
	}
}

func TestSigningEndToEnd(t *testing.T) {
	wrapper := helper.LoadPresignSavedData(0)
	n := len(wrapper.Outputs)
	batchSize := len(wrapper.Outputs[0])

	// Batch should have the same set of party ids.
	pIDs := wrapper.Outputs[0][0].PartyIds
	outCh := make(chan *common.TssMessage)
	workers := make([]worker.Worker, n)
	done := make(chan bool)
	finishedWorkerCount := 0
	signingMsg := "This is a test"

	outputs := make([][]*libCommon.SignatureData, len(pIDs)) // n * batchSize
	outputLock := &sync.Mutex{}

	for i := 0; i < n; i++ {
		request := &types.WorkRequest{
			WorkId:     "Signing0",
			WorkType:   types.EcdsaSigning,
			BatchSize:  batchSize,
			AllParties: helper.CopySortedPartyIds(pIDs),
			Threshold:  len(pIDs) - 1,
			Message:    signingMsg,
			N:          n,
		}

		workerIndex := i

		worker := NewSigningWorker(
			batchSize,
			request,
			pIDs[i],
			helper.NewTestDispatcher(outCh, 0, 0),
			mockDbForSigning(pIDs, request.WorkId, request.BatchSize),
			&helper.MockWorkerCallback{
				OnWorkSigningFinishedFunc: func(request *types.WorkRequest, data []*libCommon.SignatureData) {
					outputLock.Lock()
					defer outputLock.Unlock()

					outputs[workerIndex] = data
					finishedWorkerCount += 1
					if finishedWorkerCount == n {
						done <- true
					}
				},

				GetAvailablePresignsFunc: func(batchSize int, n int, pids []*tss.PartyID) ([]string, []*tss.PartyID) {
					return make([]string, batchSize), pids
				},

				GetPresignOutputsFunc: func(presignIds []string) []*presign.LocalPresignData {
					return wrapper.Outputs[workerIndex]
				},
			},
			10*time.Minute,
		)

		workers[i] = worker
	}

	// Start all workers
	startAllWorkers(workers)

	// Run all workers
	runAllWorkers(workers, outCh, done)

	// Verify signature
	verifySignature(t, signingMsg, outputs, wrapper)
}

func TestSigning_PreExecutionTimeout(t *testing.T) {
	wrapper := helper.LoadPresignSavedData(0)
	n := len(wrapper.Outputs)
	batchSize := len(wrapper.Outputs[0])

	// Batch should have the same set of party ids.
	pIDs := wrapper.Outputs[0][0].PartyIds
	outCh := make(chan *common.TssMessage, 4)
	workers := make([]worker.Worker, n)
	done := make(chan bool)
	signingMsg := "This is a test"
	var numFailedWorkers uint32

	for i := 0; i < n; i++ {
		request := &types.WorkRequest{
			WorkId:     fmt.Sprintf("Signing_0"),
			WorkType:   types.EcdsaSigning,
			BatchSize:  batchSize,
			AllParties: helper.CopySortedPartyIds(pIDs),
			Threshold:  len(pIDs) - 1,
			Message:    signingMsg,
			N:          n,
		}

		worker := NewSigningWorker(
			batchSize,
			request,
			pIDs[i],
			helper.NewTestDispatcher(outCh, 3*time.Second+1, 0),
			mockDbForSigning(pIDs, request.WorkId, request.BatchSize),
			&helper.MockWorkerCallback{
				OnWorkFailedFunc: func(request *types.WorkRequest) {
					if n := atomic.AddUint32(&numFailedWorkers, 1); n == 4 {
						done <- true
					}
				},
			},
			10*time.Minute,
		)

		workers[i] = worker
	}

	// Start all workers
	startAllWorkers(workers)

	// Run all workers
	runAllWorkers(workers, outCh, done)

	assert.EqualValues(t, 4, numFailedWorkers)
}

func TestSigning_ExecutionTimeout(t *testing.T) {
	wrapper := helper.LoadPresignSavedData(0)
	n := len(wrapper.Outputs)
	batchSize := len(wrapper.Outputs[0])

	// Batch should have the same set of party ids.
	pIDs := wrapper.Outputs[0][0].PartyIds
	outCh := make(chan *common.TssMessage, 4)
	workers := make([]worker.Worker, n)
	done := make(chan bool)
	signingMsg := "This is a test"
	var numFailedWorkers uint32

	for i := 0; i < n; i++ {
		request := &types.WorkRequest{
			WorkId:     fmt.Sprintf("Signing_0"),
			WorkType:   types.EcdsaSigning,
			BatchSize:  batchSize,
			AllParties: helper.CopySortedPartyIds(pIDs),
			Threshold:  len(pIDs) - 1,
			Message:    signingMsg,
			N:          n,
		}

		worker := NewSigningWorker(
			batchSize,
			request,
			pIDs[i],
			helper.NewTestDispatcher(outCh, 0, 2*time.Second+1),
			mockDbForSigning(pIDs, request.WorkId, request.BatchSize),
			&helper.MockWorkerCallback{
				OnWorkFailedFunc: func(request *types.WorkRequest) {
					if n := atomic.AddUint32(&numFailedWorkers, 1); n == 4 {
						done <- true
					}
				},
			},
			time.Second,
		)

		workers[i] = worker
	}

	// Start all workers
	startAllWorkers(workers)

	// Run all workers
	runAllWorkers(workers, outCh, done)

	assert.EqualValues(t, 4, numFailedWorkers)
}

func verifySignature(t *testing.T, msg string, outputs [][]*libCommon.SignatureData, wrapper *helper.PresignDataWrapper) {
	// Loop every single element in the batch
	for j := range outputs[0] {
		// Verify all workers have the same signature.
		for i := range outputs {
			assert.Equal(t, outputs[i][j].R, outputs[0][j].R)
			assert.Equal(t, outputs[i][j].S, outputs[0][j].S)
		}

		pubX := wrapper.Outputs[0][0].ECDSAPub.X()
		pubY := wrapper.Outputs[0][0].ECDSAPub.Y()
		R := new(big.Int).SetBytes(outputs[0][j].R)
		S := new(big.Int).SetBytes(outputs[0][j].S)

		// Verify that the signature is valid
		pk := ecdsa.PublicKey{
			Curve: tss.EC(),
			X:     pubX,
			Y:     pubY,
		}
		ok := ecdsa.Verify(&pk, []byte(msg), R, S)
		assert.True(t, ok, "ecdsa verify must pass")
	}
}
