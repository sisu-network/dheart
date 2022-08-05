package worker

import (
	"bytes"

	"crypto/ecdsa"
	"fmt"
	"math/big"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	eckeygen "github.com/sisu-network/tss-lib/ecdsa/keygen"

	ecommon "github.com/ethereum/go-ethereum/common"
	etypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/sisu-network/dheart/core/components"
	"github.com/sisu-network/dheart/core/config"
	"github.com/sisu-network/dheart/db"
	"github.com/sisu-network/dheart/types/common"
	"github.com/sisu-network/dheart/worker/types"
	"github.com/sisu-network/lib/log"
	libCommon "github.com/sisu-network/tss-lib/common"
	ecsigning "github.com/sisu-network/tss-lib/ecdsa/signing"
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

	return &db.MockDatabase{
		GetAvailablePresignShortFormFunc: func() ([]string, []string, error) {
			return presignIds, pidStrings, nil
		},

		LoadPresignFunc: func(presignIds []string) ([]*ecsigning.SignatureData_OneRoundData, error) {
			return make([]*ecsigning.SignatureData_OneRoundData, len(presignIds)), nil
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

func TestEcWorkerSigning_EndToEnd(t *testing.T) {
	wrapper := LoadEcPresignSavedData()
	n := len(wrapper.Outputs)

	// Batch should have the same set of party ids.
	pIDs := wrapper.PIDs

	outCh := make(chan *common.TssMessage)
	workers := make([]Worker, n)
	done := make(chan bool)
	finishedWorkerCount := 0
	ethTx := generateEthTx()
	signer := etypes.NewLondonSigner(big.NewInt(1))
	hash := signer.Hash(ethTx)
	hashBytes := hash[:]
	signingMsgs := [][]byte{hashBytes}

	outputs := make([][]*libCommon.ECSignature, len(pIDs)) // n * batchSize
	outputLock := &sync.Mutex{}

	for i := 0; i < n; i++ {
		request := types.NewEcSigningRequest(
			"Signing0",
			CopySortedPartyIds(pIDs),
			len(pIDs)-1,
			signingMsgs,
			[]string{"eth"},
			wrapper.KeygenOutputs[i],
		)

		workerIndex := i

		worker := NewSigningWorker(
			request,
			pIDs[i],
			NewTestDispatcher(outCh, 0, 0),
			mockDbForSigning(pIDs, request.WorkId, request.BatchSize),
			&MockWorkerCallback{
				OnWorkerResultFunc: func(request *types.WorkRequest, result *WorkerResult) {
					outputLock.Lock()
					defer outputLock.Unlock()

					outputs[workerIndex] = GetEcSigningOutputs(result.JobResults)
					finishedWorkerCount += 1
					if finishedWorkerCount == n {
						done <- true
					}
				},

				GetPresignOutputsFunc: func(presignIds []string) []*ecsigning.SignatureData_OneRoundData {
					return []*ecsigning.SignatureData_OneRoundData{wrapper.Outputs[workerIndex]}
				},
			},
			config.NewDefaultTimeoutConfig(),
			1,
			&components.MockAvailablePresigns{
				GetAvailablePresignsFunc: func(batchSize int, n int, allPids map[string]*tss.PartyID) ([]string, []*tss.PartyID) {
					return make([]string, batchSize), flattenPidMaps(allPids)
				},
			},
		)

		workers[i] = worker
	}

	// Start all workers
	startAllWorkers(workers)

	// Run all workers
	runAllWorkers(workers, outCh, done)

	// Verify signature
	verifySignature(t, signingMsgs, outputs, wrapper.KeygenOutputs[0].ECDSAPub.X(), wrapper.KeygenOutputs[0].ECDSAPub.Y())

	// Verify that this ETH transaction is correctly signed
	verifyEthSignature(t, hashBytes, outputs[0][0], wrapper.Outputs[0], wrapper.KeygenOutputs[0])
}

func TestEcWorkerSigning_PresignAndSign(t *testing.T) {
	n := 4

	// Batch should have the same set of party ids.
	pIDs := GetTestPartyIds(n)
	presignInputs := LoadEcKeygenSavedData(pIDs)
	outCh := make(chan *common.TssMessage)
	workers := make([]Worker, n)
	done := make(chan bool)
	finishedWorkerCount := 0
	signingMsgs := [][]byte{[]byte("This is a test"), []byte("another message")}

	outputs := make([][]*libCommon.ECSignature, len(pIDs)) // n * batchSize
	outputLock := &sync.Mutex{}

	for i := 0; i < n; i++ {
		request := types.NewEcSigningRequest(
			"Signing0",
			CopySortedPartyIds(pIDs),
			len(pIDs)-1,
			signingMsgs,
			[]string{"eth"},
			presignInputs[i],
		)

		workerIndex := i
		cfg := config.NewDefaultTimeoutConfig()
		cfg.SelectionLeaderTimeout = time.Second * 2

		worker := NewSigningWorker(
			request,
			pIDs[i],
			NewTestDispatcher(outCh, 0, 0),
			mockDbForSigning(pIDs, request.WorkId, request.BatchSize),
			&MockWorkerCallback{
				OnWorkerResultFunc: func(request *types.WorkRequest, result *WorkerResult) {
					outputLock.Lock()
					defer outputLock.Unlock()

					outputs[workerIndex] = GetEcSigningOutputs(result.JobResults)
					finishedWorkerCount += 1
					if finishedWorkerCount == n {
						done <- true
					}
				},

				GetPresignOutputsFunc: func(presignIds []string) []*ecsigning.SignatureData_OneRoundData {
					return nil
				},
			},
			cfg,
			1,
			&components.MockAvailablePresigns{
				GetAvailablePresignsFunc: func(batchSize int, n int, allPids map[string]*tss.PartyID) ([]string, []*tss.PartyID) {
					return nil, nil
				},
			},
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

func TestEcWorkerSigning_PreExecutionTimeout(t *testing.T) {
	wrapper := LoadEcPresignSavedData()
	n := len(wrapper.Outputs)
	pIDs := wrapper.PIDs

	outCh := make(chan *common.TssMessage, 4)
	workers := make([]Worker, n)
	done := make(chan bool)
	signingMsg := "This is a test"
	var numFailedWorkers uint32

	for i := 0; i < n; i++ {
		request := types.NewEcSigningRequest(
			"Signing0",
			CopySortedPartyIds(pIDs),
			len(pIDs)-1,
			[][]byte{[]byte(signingMsg)},
			[]string{"eth"},
			nil,
		)

		cfg := config.NewDefaultTimeoutConfig()
		cfg.SelectionLeaderTimeout = time.Second * 2

		worker := NewSigningWorker(
			request,
			pIDs[i],
			NewTestDispatcher(outCh, cfg.SelectionLeaderTimeout+1*time.Second, 0),
			mockDbForSigning(pIDs, request.WorkId, request.BatchSize),
			&MockWorkerCallback{
				OnWorkFailedFunc: func(request *types.WorkRequest) {
					if n := atomic.AddUint32(&numFailedWorkers, 1); n == 4 {
						done <- true
					}
				},
			},
			cfg,
			1,
			&components.MockAvailablePresigns{},
		)

		workers[i] = worker
	}

	// Start all workers
	startAllWorkers(workers)

	// Run all workers
	runAllWorkers(workers, outCh, done)

	assert.EqualValues(t, 4, numFailedWorkers)
}

func TestEcWorkerSigning_ExecutionTimeout(t *testing.T) {
	wrapper := LoadEcPresignSavedData()
	n := len(wrapper.Outputs)
	pIDs := wrapper.PIDs

	outCh := make(chan *common.TssMessage, 4)
	workers := make([]Worker, n)
	done := make(chan bool)
	signingMsg := "This is a test"
	var numFailedWorkers uint32

	for i := 0; i < n; i++ {
		request := types.NewEcSigningRequest(
			"Signing0",
			CopySortedPartyIds(pIDs),
			len(pIDs)-1,
			[][]byte{[]byte(signingMsg)},
			[]string{"eth"},
			wrapper.KeygenOutputs[i],
		)

		workerIndex := i
		cfg := config.NewDefaultTimeoutConfig()
		cfg.SigningJobTimeout = time.Second

		worker := NewSigningWorker(
			request,
			pIDs[i],
			NewTestDispatcher(outCh, 0, 2*time.Second+1),
			mockDbForSigning(pIDs, request.WorkId, request.BatchSize),
			&MockWorkerCallback{
				OnWorkFailedFunc: func(request *types.WorkRequest) {
					if n := atomic.AddUint32(&numFailedWorkers, 1); n == 4 {
						done <- true
					}
				},
				GetPresignOutputsFunc: func(presignIds []string) []*ecsigning.SignatureData_OneRoundData {
					return []*ecsigning.SignatureData_OneRoundData{wrapper.Outputs[workerIndex]}
				},
			},
			cfg,
			1,
			&components.MockAvailablePresigns{
				GetAvailablePresignsFunc: func(batchSize int, n int, allPids map[string]*tss.PartyID) ([]string, []*tss.PartyID) {
					return make([]string, batchSize), flattenPidMaps(allPids)
				},
			},
		)

		workers[i] = worker
	}

	// Start all workers
	startAllWorkers(workers)

	// Run all workers
	runAllWorkers(workers, outCh, done)

	assert.EqualValues(t, 4, numFailedWorkers)
}

func verifySignature(t *testing.T, msgs [][]byte, outputs [][]*libCommon.ECSignature, pubX, pubY *big.Int) {
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
				Curve: tss.EC(tss.EcdsaScheme),
				X:     pubX,
				Y:     pubY,
			}
			ok := ecdsa.Verify(&pk, msgs[j], R, S)
			assert.True(t, ok, "ecdsa verify must pass")
		}
	}
}

// Runs test when we have a strict threshold < n - 1.
// We need to run test multiple times to make sure we do not have concurrent issue.
func TestEcWorkerSigning_Threshold(t *testing.T) {
	for i := 0; i < 5; i++ {
		doTestThreshold(t)
	}
}

func doTestThreshold(t *testing.T) {
	n := 4
	wrapper := LoadEcPresignSavedData()
	threshold := 3

	if len(wrapper.Outputs) != threshold+1 {
		t.Fatal(fmt.Errorf("Signing input is not correct!"))
	}

	// This is pids of all parties taking part in the presign. Not all are guaranteed to be selected.
	pIDs := CopySortedPartyIds(wrapper.PIDs)
	presignDataMap := make(map[string]*ecsigning.SignatureData_OneRoundData)
	selectedPids := make([]*tss.PartyID, 0)
	for _, output := range wrapper.Outputs {
		for _, partyId := range pIDs {
			if output.PartyId == partyId.Id {
				selectedPids = append(selectedPids, partyId)
				presignDataMap[partyId.Id] = output
				break
			}
		}
	}

	if len(selectedPids) != threshold+1 {
		panic("selectedPids does not match threshold + 1")
	}

	selectedPids = tss.SortPartyIDs(selectedPids)

	outCh := make(chan *common.TssMessage)
	workers := make([]Worker, n)
	done := make(chan bool)
	finishedWorkerCount := 0
	ethTx := generateEthTx()
	signer := etypes.NewLondonSigner(big.NewInt(1))
	hash := signer.Hash(ethTx)
	hashBytes := hash[:]
	signingMsgs := [][]byte{hashBytes}

	outputs := make([][]*libCommon.ECSignature, 0) // n * batchSize
	outputLock := &sync.Mutex{}

	for i := 0; i < n; i++ {
		request := types.NewEcSigningRequest(
			"Signing0",
			CopySortedPartyIds(pIDs),
			threshold,
			signingMsgs,
			[]string{"eth"},
			wrapper.KeygenOutputs[i],
		)

		myPid := pIDs[i]

		worker := NewSigningWorker(
			request,
			pIDs[i],
			NewTestDispatcher(outCh, 0, 0),
			mockDbForSigning(pIDs, request.WorkId, request.BatchSize),
			&MockWorkerCallback{
				OnWorkerResultFunc: func(request *types.WorkRequest, result *WorkerResult) {
					outputLock.Lock()
					defer outputLock.Unlock()

					outputs = append(outputs, GetEcSigningOutputs(result.JobResults))
					finishedWorkerCount += 1
					if finishedWorkerCount == n {
						done <- true
					}
				},

				GetPresignOutputsFunc: func(presignIds []string) []*ecsigning.SignatureData_OneRoundData {
					return []*ecsigning.SignatureData_OneRoundData{presignDataMap[myPid.Id]}
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
			config.NewDefaultTimeoutConfig(),
			1,
			&components.MockAvailablePresigns{
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
			},
		)

		workers[i] = worker
	}

	// Start all workers
	startAllWorkers(workers)

	// Run all workers
	runAllWorkers(workers, outCh, done)

	// Verify signature
	verifySignature(t, signingMsgs, outputs, wrapper.KeygenOutputs[0].ECDSAPub.X(), wrapper.KeygenOutputs[0].ECDSAPub.Y())

	// Verify that this ETH transaction is correctly signed
	verifyEthSignature(t, hashBytes, outputs[0][0], wrapper.Outputs[0], wrapper.KeygenOutputs[0])
}

func verifyEthSignature(t *testing.T, hash []byte, output *libCommon.ECSignature,
	presignData *ecsigning.SignatureData_OneRoundData, keygenOutput *eckeygen.LocalPartySaveData) {
	signature := output.Signature
	signature = append(signature, output.SignatureRecovery[0])

	sigPublicKey, err := crypto.Ecrecover(hash, signature)
	if err != nil {
		t.Fail()
	}

	publicKeyECDSA := ecdsa.PublicKey{
		Curve: tss.EC(tss.EcdsaScheme),
		X:     keygenOutput.ECDSAPub.X(),
		Y:     keygenOutput.ECDSAPub.Y(),
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
