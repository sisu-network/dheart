package ecdsa

import (
	"crypto/ecdsa"
	"math/big"
	"testing"

	"github.com/sisu-network/dheart/types/common"
	"github.com/sisu-network/dheart/worker"
	"github.com/sisu-network/dheart/worker/helper"
	libCommon "github.com/sisu-network/tss-lib/common"
	"github.com/sisu-network/tss-lib/tss"
	"github.com/stretchr/testify/assert"
)

func TestSigningEndToEnd(t *testing.T) {
	wrapper := helper.LoadPresignSavedData(0)
	n := len(wrapper.Outputs)
	batchSize := len(wrapper.Outputs[0])

	// Batch should have the same set of party ids.
	pIDs := wrapper.Outputs[0][0].PartyIds
	p2pCtx := tss.NewPeerContext(pIDs)
	outCh := make(chan *common.TssMessage)
	errCh := make(chan error)
	workers := make([]worker.Worker, n)
	done := make(chan bool)
	finishedWorkerCount := 0
	signingMsg := "This is a test"

	outputs := make([][]*libCommon.SignatureData, len(pIDs)) // n * batchSize
	cb := func(workerIndex int, workerId string, data []*libCommon.SignatureData) {
		outputs[workerIndex] = data
		finishedWorkerCount += 1

		if finishedWorkerCount == n {
			done <- true
		}
	}

	for i := 0; i < n; i++ {
		params := tss.NewParameters(p2pCtx, pIDs[i], len(pIDs), n-1)
		worker := NewSigningWorker(
			"Signing0",
			batchSize,
			pIDs,
			pIDs,
			pIDs[i],
			params,
			signingMsg,
			wrapper.Outputs[i],
			helper.NewTestDispatcher(outCh),
			errCh,
			helper.NewTestSigningCallback(i, cb),
		)

		workers[i] = worker
	}

	// Start all workers
	startAllWorkers(workers)

	// Run all workers
	runAllWorkers(workers, outCh, errCh, done)

	// Verify signature
	verifySignature(t, signingMsg, outputs, wrapper)
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
