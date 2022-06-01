package worker

import (
	"testing"
	"time"

	"github.com/sisu-network/tss-lib/tss"
)

func TestEcJob_Keygen(t *testing.T) {
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
		jobs[i] = NewEcKeygenJob("Keygen0", i, pIDs, params, preparams, cbs[i], time.Second*15)
	}

	runJobs(t, jobs, cbs, true)
}

func TestEcJob_Presign(t *testing.T) {
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
	n := 4
	threshold := 1

	jobs := make([]*Job, n)
	cbs := make([]*MockJobCallback, n)

	for i := 0; i < n; i++ {
		cbs[i] = &MockJobCallback{}
	}

	savedPresigns := LoadEcPresignSavedData(0)
	pIDs := savedPresigns.PIDs

	for i := 0; i < n; i++ {
		p2pCtx := tss.NewPeerContext(pIDs)
		params := tss.NewParameters(p2pCtx, pIDs[i], len(pIDs), threshold)
		jobs[i] = NewEcSigningJob("Signinng0", i, pIDs, params, "Test message",
			savedPresigns.Outputs[i][0], cbs[i], time.Second*15)
	}

	runJobs(t, jobs, cbs, true)
}
