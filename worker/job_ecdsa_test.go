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
		jobs[i] = NewEcSigningJob("Presign0", i, pIDs, params, "", *presignInputs[i], nil, cbs[i], time.Second*10)
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
	presignInputs := LoadEcKeygenSavedData(pIDs)

	msg := "Test message"

	for i := 0; i < n; i++ {
		p2pCtx := tss.NewPeerContext(pIDs)
		params := tss.NewParameters(p2pCtx, pIDs[i], len(pIDs), threshold)
		jobs[i] = NewEcSigningJob("Signinng0", i, pIDs, params, msg, *presignInputs[i],
			savedPresigns.Outputs[i][0], cbs[i], time.Second*15)
	}

	runJobs(t, jobs, cbs, true)

	// Verify that all jobs produce the same signature
	for _, result := range results {
		require.Equal(t, result.EcSigning.Signature, results[0].EcSigning.Signature)
		require.Equal(t, result.EcSigning.Signature.SignatureRecovery, results[0].EcSigning.Signature.SignatureRecovery)
		require.Equal(t, result.EcSigning.Signature.R, results[0].EcSigning.Signature.R)
		require.Equal(t, result.EcSigning.Signature.S, results[0].EcSigning.Signature.S)
	}

	// Verify ecdsa signature
	pkX, pkY := presignInputs[0].ECDSAPub.X(), presignInputs[0].ECDSAPub.Y()
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
