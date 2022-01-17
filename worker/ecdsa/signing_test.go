package ecdsa

import (
	"bytes"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	ecommon "github.com/ethereum/go-ethereum/common"
	etypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/sisu-network/dheart/db"
	"github.com/sisu-network/dheart/types/common"
	"github.com/sisu-network/dheart/worker"
	"github.com/sisu-network/dheart/worker/helper"
	"github.com/sisu-network/dheart/worker/types"
	"github.com/sisu-network/lib/log"
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

func generateEthTx() *etypes.Transaction {
	nonce := 0

	value := big.NewInt(1000000000000000000) // in wei (1 eth)
	gasLimit := uint64(21000)                // in units
	gasPrice := big.NewInt(50)

	toAddress := ecommon.HexToAddress("0x4592d8f8d7b001e72cb26a73e4fa1806a51ac79d")
	var data []byte
	tx := etypes.NewTransaction(uint64(nonce), toAddress, value, gasLimit, gasPrice, data)

	return tx
}

func TestSigningEndToEnd(t *testing.T) {
	wrapper := helper.LoadPresignSavedData(0)
	n := len(wrapper.Outputs)

	// Batch should have the same set of party ids.
	pIDs := wrapper.Outputs[0][0].PartyIds
	outCh := make(chan *common.TssMessage)
	workers := make([]worker.Worker, n)
	done := make(chan bool)
	finishedWorkerCount := 0
	ethTx := generateEthTx()
	signer := etypes.NewEIP2930Signer(big.NewInt(1))
	hash := signer.Hash(ethTx)
	hashBytes := hash[:]
	signingMsgs := []string{string(hashBytes)}

	outputs := make([][]*libCommon.SignatureData, len(pIDs)) // n * batchSize
	outputLock := &sync.Mutex{}

	for i := 0; i < n; i++ {
		request := types.NewSigningRequest(
			"Signing0",
			helper.CopySortedPartyIds(pIDs),
			len(pIDs)-1,
			signingMsgs,
			[]string{"eth"},
			nil,
		)

		workerIndex := i

		worker := NewSigningWorker(
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

				GetAvailablePresignsFunc: func(batchSize int, n int, allPids map[string]*tss.PartyID) ([]string, []*tss.PartyID) {
					return make([]string, batchSize), flattenPidMaps(allPids)
				},

				GetPresignOutputsFunc: func(presignIds []string) []*presign.LocalPresignData {
					return wrapper.Outputs[workerIndex]
				},
			},
			10*time.Minute,
			1,
		)

		workers[i] = worker
	}

	// Start all workers
	startAllWorkers(workers)

	// Run all workers
	runAllWorkers(workers, outCh, done)

	// Verify signature
	verifySignature(t, signingMsgs, outputs, wrapper.Outputs[0][0].ECDSAPub.X(), wrapper.Outputs[0][0].ECDSAPub.Y())

	// Verify that this ETH transaction is correctly signed
	verifyEthSignature(t, hashBytes, outputs[0][0], wrapper.Outputs[0][0])
}

func TestSigning_PresignAndSign(t *testing.T) {
	n := 4

	// Batch should have the same set of party ids.
	pIDs := helper.GetTestPartyIds(n)
	presignInputs := helper.LoadKeygenSavedData(pIDs)
	outCh := make(chan *common.TssMessage)
	workers := make([]worker.Worker, n)
	done := make(chan bool)
	finishedWorkerCount := 0
	signingMsgs := []string{"This is a test", "another message"}

	outputs := make([][]*libCommon.SignatureData, len(pIDs)) // n * batchSize
	outputLock := &sync.Mutex{}

	for i := 0; i < n; i++ {
		request := types.NewSigningRequest(
			"Signing0",
			helper.CopySortedPartyIds(pIDs),
			len(pIDs)-1,
			signingMsgs,
			[]string{"eth"},
			presignInputs[i],
		)

		workerIndex := i

		worker := NewSigningWorker(
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

				GetAvailablePresignsFunc: func(batchSize int, n int, allPids map[string]*tss.PartyID) ([]string, []*tss.PartyID) {
					return nil, nil
				},

				GetPresignOutputsFunc: func(presignIds []string) []*presign.LocalPresignData {
					return nil
				},
			},
			10*time.Minute,
			1,
		)

		workers[i] = worker
	}

	// Start all workers
	startAllWorkers(workers)

	// Run all workers
	runAllWorkers(workers, outCh, done)

	// Verify signature
	verifySignature(t, signingMsgs, outputs, nil, nil)
}

func TestSigning_PreExecutionTimeout(t *testing.T) {
	wrapper := helper.LoadPresignSavedData(0)
	n := len(wrapper.Outputs)

	// Batch should have the same set of party ids.
	pIDs := wrapper.Outputs[0][0].PartyIds
	outCh := make(chan *common.TssMessage, 4)
	workers := make([]worker.Worker, n)
	done := make(chan bool)
	signingMsg := "This is a test"
	var numFailedWorkers uint32

	for i := 0; i < n; i++ {
		request := types.NewSigningRequest(
			"Signing0",
			helper.CopySortedPartyIds(pIDs),
			len(pIDs)-1,
			[]string{signingMsg},
			[]string{"eth"},
			nil,
		)

		worker := NewSigningWorker(
			request,
			pIDs[i],
			helper.NewTestDispatcher(outCh, PreExecutionRequestWaitTime+1*time.Second, 0),
			mockDbForSigning(pIDs, request.WorkId, request.BatchSize),
			&helper.MockWorkerCallback{
				OnWorkFailedFunc: func(request *types.WorkRequest) {
					if n := atomic.AddUint32(&numFailedWorkers, 1); n == 4 {
						done <- true
					}
				},
			},
			10*time.Minute,
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

func TestSigning_ExecutionTimeout(t *testing.T) {
	wrapper := helper.LoadPresignSavedData(0)
	n := len(wrapper.Outputs)

	// Batch should have the same set of party ids.
	pIDs := wrapper.Outputs[0][0].PartyIds
	outCh := make(chan *common.TssMessage, 4)
	workers := make([]worker.Worker, n)
	done := make(chan bool)
	signingMsg := "This is a test"
	var numFailedWorkers uint32

	for i := 0; i < n; i++ {
		request := types.NewSigningRequest(
			"Signing0",
			helper.CopySortedPartyIds(pIDs),
			len(pIDs)-1,
			[]string{signingMsg},
			[]string{"eth"},
			nil,
		)

		workerIndex := i

		worker := NewSigningWorker(
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

				GetAvailablePresignsFunc: func(batchSize int, n int, allPids map[string]*tss.PartyID) ([]string, []*tss.PartyID) {
					return make([]string, batchSize), flattenPidMaps(allPids)
				},

				GetPresignOutputsFunc: func(presignIds []string) []*presign.LocalPresignData {
					return wrapper.Outputs[workerIndex]
				},
			},
			time.Second,
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

func verifySignature(t *testing.T, msgs []string, outputs [][]*libCommon.SignatureData, pubX, pubY *big.Int) {
	// Loop every single element in the batch
	for j := range outputs[0] {
		// Verify all workers have the same signature.
		for i := range outputs {
			assert.Equal(t, outputs[i][j].R, outputs[0][j].R)
			assert.Equal(t, outputs[i][j].S, outputs[0][j].S)
		}

		if pubX != nil && pubY != nil {
			R := new(big.Int).SetBytes(outputs[0][j].R)
			S := new(big.Int).SetBytes(outputs[0][j].S)

			// Verify that the signature is valid
			pk := ecdsa.PublicKey{
				Curve: tss.EC(),
				X:     pubX,
				Y:     pubY,
			}
			ok := ecdsa.Verify(&pk, []byte(msgs[j]), R, S)
			assert.True(t, ok, "ecdsa verify must pass")
		}
	}
}

// Runs test when we have a strict threshold < n - 1.
// We need to run test multiple times to make sure we do not have concurrent issue.
func TestSigning_Threshold(t *testing.T) {
	for i := 0; i < 5; i++ {
		doTestThreshold(t)
	}
}

func doTestThreshold(t *testing.T) {
	n := 4
	wrapper := helper.LoadPresignSavedData(2)
	threshold := 2

	if len(wrapper.Outputs) != threshold+1 {
		t.Fatal(fmt.Errorf("Signing input is not correct!"))
	}

	selectedPids := wrapper.Outputs[0][0].PartyIds

	// Batch should have the same set of party ids.
	pIDs := helper.GetTestPartyIds(n)

	outCh := make(chan *common.TssMessage)
	workers := make([]worker.Worker, n)
	done := make(chan bool)
	finishedWorkerCount := 0
	ethTx := generateEthTx()
	signer := etypes.NewEIP2930Signer(big.NewInt(1))
	hash := signer.Hash(ethTx)
	hashBytes := hash[:]
	signingMsgs := []string{string(hashBytes)}

	outputs := make([][]*libCommon.SignatureData, 0) // n * batchSize
	outputLock := &sync.Mutex{}

	for i := 0; i < n; i++ {
		request := types.NewSigningRequest(
			"Signing0",
			helper.CopySortedPartyIds(pIDs),
			threshold,
			signingMsgs,
			[]string{"eth"},
			nil,
		)

		// workerIndex := i
		myPid := pIDs[i]

		worker := NewSigningWorker(
			request,
			pIDs[i],
			helper.NewTestDispatcher(outCh, 0, 0),
			mockDbForSigning(pIDs, request.WorkId, request.BatchSize),
			&helper.MockWorkerCallback{
				OnWorkSigningFinishedFunc: func(request *types.WorkRequest, data []*libCommon.SignatureData) {
					outputLock.Lock()
					defer outputLock.Unlock()

					outputs = append(outputs, data)
					finishedWorkerCount += 1
					if finishedWorkerCount == n {
						done <- true
					}
				},

				GetAvailablePresignsFunc: func(batchSize int, n int, allPids map[string]*tss.PartyID) ([]string, []*tss.PartyID) {
					if len(allPids) < len(wrapper.Outputs) {
						return []string{}, []*tss.PartyID{}
					}

					found := true
					for _, selected := range selectedPids {
						if _, ok := allPids[selected.Id]; !ok {
							found = false
							break
						}
					}

					if !found {
						return []string{}, []*tss.PartyID{}
					}

					return make([]string, batchSize), selectedPids
				},

				GetPresignOutputsFunc: func(presignIds []string) []*presign.LocalPresignData {
					for i := 0; i < len(selectedPids); i++ {
						if selectedPids[i].Id == myPid.Id {
							return wrapper.Outputs[i]
						}
					}

					return nil
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
			10*time.Minute,
			1,
		)

		workers[i] = worker
	}

	// Start all workers
	startAllWorkers(workers)

	// Run all workers
	runAllWorkers(workers, outCh, done)

	// Verify signature
	verifySignature(t, signingMsgs, outputs, wrapper.Outputs[0][0].ECDSAPub.X(), wrapper.Outputs[0][0].ECDSAPub.Y())

	// Verify that this ETH transaction is correctly signed
	verifyEthSignature(t, hashBytes, outputs[0][0], wrapper.Outputs[0][0])
}

func verifyEthSignature(t *testing.T, hash []byte, output *libCommon.SignatureData, presignData *presign.LocalPresignData) {
	signature := output.Signature
	signature = append(signature, output.SignatureRecovery[0])

	sigPublicKey, err := crypto.Ecrecover(hash, signature)
	if err != nil {
		t.Fail()
	}

	publicKeyECDSA := ecdsa.PublicKey{
		Curve: tss.EC(),
		X:     presignData.ECDSAPub.X(),
		Y:     presignData.ECDSAPub.Y(),
	}
	publicKeyBytes := crypto.FromECDSAPub(&publicKeyECDSA)

	if bytes.Compare(sigPublicKey, publicKeyBytes) != 0 {
		panic("Pubkey does not match")
	}

	matches := bytes.Equal(sigPublicKey, publicKeyBytes)
	if !matches {
		panic("Reconstructed pubkey does not match pubkey")
	}
	log.Info("ETH signature is correct")
}
