package worker

import (
	"testing"
	"time"

	"github.com/decred/dcrd/dcrec/edwards/v2"
	edkeygen "github.com/sisu-network/tss-lib/eddsa/keygen"
	"github.com/sisu-network/tss-lib/tss"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEdJob_Keygen(t *testing.T) {
	t.Parallel()

	// Change this to 15 when re-generating & saving keygen output
	n := 6
	threshold := 1
	jobs := make([]*Job, n)
	cbs := make([]*MockJobCallback, n)
	outputs := make([]*edkeygen.LocalPartySaveData, n)
	pIDs := GetTestPartyIds(n)

	for i := 0; i < n; i++ {
		index := i
		cbs[index] = &MockJobCallback{}
		cbs[index].OnJobResultFunc = func(job *Job, result JobResult) {
			outputs[index] = result.EdKeygen
		}
	}

	for i := 0; i < n; i++ {
		p2pCtx := tss.NewPeerContext(pIDs)
		params := tss.NewParameters(p2pCtx, pIDs[i], len(pIDs), threshold)
		jobs[i] = NewEdKeygenJob("Keygen0", i, pIDs, params, cbs[i], time.Second*120)
	}

	runJobs(t, jobs, cbs, true)

	// Uncomment this line to save keygen outputs.
	// SaveEdKeygenOutput(outputs)
}

func TestEdJob_Signing(t *testing.T) {
	n := 6
	threshold := 1
	jobs := make([]*Job, n)
	cbs := make([]*MockJobCallback, n)

	pIDs := GetTestPartyIds(n)

	results := make([]JobResult, n)

	for i := 0; i < n; i++ {
		index := i
		cbs[index] = &MockJobCallback{}

		cbs[index].OnJobResultFunc = func(job *Job, result JobResult) {
			results[index] = result
		}
	}

	keygenOutputs := LoadEdKeygenSavedData(pIDs)

	msgBytes := []byte("Test")

	for i := 0; i < n; i++ {
		p2pCtx := tss.NewPeerContext(pIDs)
		params := tss.NewParameters(p2pCtx, pIDs[i], len(pIDs), threshold)
		jobs[i] = NewEdSigningJob("EdSign0", i, pIDs, params, msgBytes, *keygenOutputs[i], cbs[i], time.Second*10)
	}

	runJobs(t, jobs, cbs, true)

	// Verify that all jobs produce the same signature
	for _, result := range results {
		require.Equal(t, result.EdSigning.Signature.Signature, results[0].EdSigning.Signature.Signature)
	}

	// Verify eddsa signature
	pkX, pkY := keygenOutputs[0].EDDSAPub.X(), keygenOutputs[0].EDDSAPub.Y()
	pk := edwards.PublicKey{
		Curve: tss.EC("eddsa"),
		X:     pkX,
		Y:     pkY,
	}

	newSig, err := edwards.ParseSignature(results[0].EdSigning.Signature.Signature)
	if err != nil {
		println("new sig error, ", err.Error())
	}

	ok := edwards.Verify(&pk, msgBytes, newSig.R, newSig.S)
	assert.True(t, ok, "eddsa verify must pass")
}
