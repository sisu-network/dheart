package worker

import (
	"testing"
	"time"

	edkeygen "github.com/sisu-network/tss-lib/eddsa/keygen"
	"github.com/sisu-network/tss-lib/tss"
)

func TestEdJob_Keygen(t *testing.T) {
	n := 6
	threshold := 1
	jobs := make([]*Job, n)
	cbs := make([]*MockJobCallback, n)

	pIDs := GetTestPartyIds(n)

	outputs := make([]*edkeygen.LocalPartySaveData, n)

	for i := 0; i < n; i++ {
		index := i
		cbs[index] = &MockJobCallback{}
		cbs[index].OnJobResultFunc = func(job *Job, result JobResult) {
			outputs[index] = &result.EdKeygen
		}
	}

	for i := 0; i < n; i++ {
		p2pCtx := tss.NewPeerContext(pIDs)
		params := tss.NewParameters(p2pCtx, pIDs[i], len(pIDs), threshold)
		jobs[i] = NewEdKeygenJob("Keygen0", i, pIDs, params, cbs[i], time.Second*15)
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

	for i := 0; i < n; i++ {
		index := i
		cbs[index] = &MockJobCallback{}
	}

	keygenOutputs := LoadEdKeygenSavedData(pIDs)

	for i := 0; i < n; i++ {
		p2pCtx := tss.NewPeerContext(pIDs)
		params := tss.NewParameters(p2pCtx, pIDs[i], len(pIDs), threshold)
		jobs[i] = NewEdSigningJob("Sign0", i, pIDs, params, []byte("Test"), *keygenOutputs[i], cbs[i], time.Second*10)
	}

	runJobs(t, jobs, cbs, true)
}
