package worker

import (
	"testing"
	"time"

	"github.com/sisu-network/tss-lib/tss"
)

func TestEdJob_Keygen(t *testing.T) {
	n := 4
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
		jobs[i] = NewEdKeygenJob("Keygen0", i, pIDs, params, cbs[i], time.Second*15)
	}

	runJobs(t, jobs, cbs, true)
}
