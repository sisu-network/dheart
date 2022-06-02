package worker

import (
	"crypto/ecdsa"
	"math/big"
	"testing"
	"time"

	"github.com/sisu-network/tss-lib/crypto"
	eckeygen "github.com/sisu-network/tss-lib/ecdsa/keygen"
	ecsigning "github.com/sisu-network/tss-lib/ecdsa/signing"
	"github.com/sisu-network/tss-lib/tss"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEcJob_Keygen(t *testing.T) {
	t.Parallel()

	n := 15
	threshold := 1

	jobs := make([]*Job, n)
	cbs := make([]*MockJobCallback, n)

	pIDs := GetTestPartyIds(n)

	outputs := make([]*eckeygen.LocalPartySaveData, n)

	for i := 0; i < n; i++ {
		index := i
		cbs[index] = &MockJobCallback{}
		cbs[index].OnJobResultFunc = func(job *Job, result JobResult) {
			outputs[index] = &result.EcKeygen
		}
	}

	for i := 0; i < n; i++ {
		p2pCtx := tss.NewPeerContext(pIDs)
		params := tss.NewParameters(p2pCtx, pIDs[i], len(pIDs), threshold)
		preparams := LoadEcPreparams(i)
		jobs[i] = NewEcKeygenJob("Keygen0", i, pIDs, params, preparams, cbs[i], time.Second*120)
	}

	runJobs(t, jobs, cbs, true)

	// Uncomment this line to save the keygen outputs
	// SaveEcKeygenOutput(outputs)
}

func TestEcJob_Presign(t *testing.T) {
	t.Parallel()

	n := 4
	threshold := 1
	jobs := make([]*Job, n)
	cbs := make([]*MockJobCallback, n)

	pIDs := GetTestPartyIds(n)
	keygenOutputs := LoadEcKeygenSavedData(pIDs)

	presignOutputs := make([]*ecsigning.SignatureData_OneRoundData, n)

	for i := 0; i < n; i++ {
		index := i
		cbs[index] = &MockJobCallback{
			OnJobResultFunc: func(job *Job, result JobResult) {
				presignOutputs[index] = result.EcSigning.OneRoundData
			},
		}
	}

	for i := 0; i < n; i++ {
		p2pCtx := tss.NewPeerContext(pIDs)
		params := tss.NewParameters(p2pCtx, pIDs[i], len(pIDs), threshold)
		jobs[i] = NewEcSigningJob("Presign0", i, pIDs, params, nil, *keygenOutputs[i], nil, cbs[i], time.Second*10)
	}

	runJobs(t, jobs, cbs, true)

	// Uncomment this line to save the presign outputs
	SaveEcPresignData(n, keygenOutputs, presignOutputs, pIDs)
}

func TestEcJob_Signing_WithPresign(t *testing.T) {
	t.Parallel()

	n := 4
	threshold := 1

	jobs := make([]*Job, n)
	cbs := make([]*MockJobCallback, n)
	results := make([]JobResult, n)

	for i := 0; i < n; i++ {
		index := i
		cbs[index] = &MockJobCallback{}
		cbs[index].OnJobResultFunc = func(job *Job, result JobResult) {
			results[index] = result
		}
	}

	savedPresigns := LoadEcPresignSavedData()
	pIDs := savedPresigns.PIDs
	keygenOutputs := savedPresigns.KeygenOutputs

	msg := "Test message"

	for i := 0; i < n; i++ {
		p2pCtx := tss.NewPeerContext(pIDs)
		params := tss.NewParameters(p2pCtx, pIDs[i], len(pIDs), threshold)
		jobs[i] = NewEcSigningJob("Signinng0", i, pIDs, params, []byte(msg), *keygenOutputs[i],
			savedPresigns.Outputs[i], cbs[i], time.Second*15)
	}

	runJobs(t, jobs, cbs, true)

	verifyEcSignature(t, msg, results, keygenOutputs[0].ECDSAPub)
}

func TestEcJob_Signing_NoPresign(t *testing.T) {
	t.Parallel()

	n := 4
	threshold := 1

	jobs := make([]*Job, n)
	cbs := make([]*MockJobCallback, n)
	results := make([]JobResult, n)

	for i := 0; i < n; i++ {
		index := i
		cbs[index] = &MockJobCallback{}
		cbs[index].OnJobResultFunc = func(job *Job, result JobResult) {
			results[index] = result
		}
	}

	pIDs := GetTestPartyIds(n)
	keygenOutputs := LoadEcKeygenSavedData(pIDs)
	msg := "Test message"

	for i := 0; i < n; i++ {
		p2pCtx := tss.NewPeerContext(pIDs)
		params := tss.NewParameters(p2pCtx, pIDs[i], len(pIDs), threshold)
		jobs[i] = NewEcSigningJob("Signinng0", i, pIDs, params, []byte(msg), *keygenOutputs[i],
			nil, cbs[i], time.Second*15)
	}

	runJobs(t, jobs, cbs, true)

	verifyEcSignature(t, msg, results, keygenOutputs[0].ECDSAPub)
}

func verifyEcSignature(t *testing.T, msg string, results []JobResult, pubkey *crypto.ECPoint) {
	// Verify that all jobs produce the same signature
	for _, result := range results {
		require.Equal(t, result.EcSigning.Signature, results[0].EcSigning.Signature)
		require.Equal(t, result.EcSigning.Signature.SignatureRecovery, results[0].EcSigning.Signature.SignatureRecovery)
		require.Equal(t, result.EcSigning.Signature.R, results[0].EcSigning.Signature.R)
		require.Equal(t, result.EcSigning.Signature.S, results[0].EcSigning.Signature.S)
	}

	// Verify ecdsa signature
	pkX, pkY := pubkey.X(), pubkey.Y()
	pk := ecdsa.PublicKey{
		Curve: tss.EC("ecdsa"),
		X:     pkX,
		Y:     pkY,
	}
	ok := ecdsa.Verify(
		&pk,
		[]byte(msg),
		new(big.Int).SetBytes(results[0].EcSigning.Signature.R),
		new(big.Int).SetBytes(results[0].EcSigning.Signature.S),
	)
	assert.True(t, ok, "ecdsa verify must pass")
}
