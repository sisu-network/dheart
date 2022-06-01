package worker

import (
	"crypto/ecdsa"
	"math/big"
	"testing"
	"time"

	"github.com/sisu-network/tss-lib/tss"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEcJob_Keygen(t *testing.T) {
	t.Parallel()

	n := 6
	threshold := 1

	jobs := make([]*Job, n)
	cbs := make([]*MockJobCallback, n)

	pIDs := GetTestPartyIds(n)

	for i := 0; i < n; i++ {
		cbs[i] = &MockJobCallback{}
	}

	for i := 0; i < n; i++ {
		p2pCtx := tss.NewPeerContext(pIDs)
		params := tss.NewParameters(p2pCtx, pIDs[i], len(pIDs), threshold)
		preparams := LoadEcPreparams(i)
		jobs[i] = NewEcKeygenJob("Keygen0", i, pIDs, params, preparams, cbs[i], time.Second*120)
	}

	runJobs(t, jobs, cbs, true)
}

func TestEcJob_Presign(t *testing.T) {
	t.Parallel()

	n := 4
	threshold := 1
	jobs := make([]*Job, n)
	cbs := make([]*MockJobCallback, n)

	pIDs := GetTestPartyIds(n)
	presignInputs := LoadEcKeygenSavedData(pIDs)

	for i := 0; i < n; i++ {
		cbs[i] = &MockJobCallback{}
	}

	for i := 0; i < n; i++ {
		p2pCtx := tss.NewPeerContext(pIDs)
		params := tss.NewParameters(p2pCtx, pIDs[i], len(pIDs), threshold)
		jobs[i] = NewEcPresignJob("Presign0", i, pIDs, params, presignInputs[i], cbs[i], time.Second*30)
	}

	runJobs(t, jobs, cbs, true)
}

func TestEcJob_Signing(t *testing.T) {
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

	savedPresigns := LoadEcPresignSavedData(0)
	pIDs := savedPresigns.PIDs

	msg := "Test message"

	for i := 0; i < n; i++ {
		p2pCtx := tss.NewPeerContext(pIDs)
		params := tss.NewParameters(p2pCtx, pIDs[i], len(pIDs), threshold)
		jobs[i] = NewEcSigningJob("Signinng0", i, pIDs, params, msg,
			savedPresigns.Outputs[i][0], cbs[i], time.Second*15)
	}

	runJobs(t, jobs, cbs, true)

	// Verify that all jobs produce the same signature
	for _, result := range results {
		require.Equal(t, result.EcSigning.Signature, results[0].EcSigning.Signature)
		require.Equal(t, result.EcSigning.SignatureRecovery, results[0].EcSigning.SignatureRecovery)
		require.Equal(t, result.EcSigning.R, results[0].EcSigning.R)
		require.Equal(t, result.EcSigning.S, results[0].EcSigning.S)
	}

	// Verify ecdsa signature
	presignData := savedPresigns.Outputs[0][0]
	pkX, pkY := presignData.ECDSAPub.X(), presignData.ECDSAPub.Y()
	pk := ecdsa.PublicKey{
		Curve: tss.EC("ecdsa"),
		X:     pkX,
		Y:     pkY,
	}
	ok := ecdsa.Verify(&pk, []byte(msg), new(big.Int).SetBytes(results[0].EcSigning.R), new(big.Int).SetBytes(results[0].EcSigning.S))
	assert.True(t, ok, "ecdsa verify must pass")
}
