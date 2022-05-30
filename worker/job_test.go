package worker

import (
	"sync"
	"testing"
	"time"

	"github.com/sisu-network/dheart/worker/helper"
	"github.com/sisu-network/tss-lib/tss"
	"github.com/stretchr/testify/require"
)

func TestJob_Presign(t *testing.T) {
	n := 4
	threshold := 1
	jobs := make([]*Job, n)
	cbs := make([]*MockJobCallback, n)

	pIDs := helper.GetTestPartyIds(n)
	presignInputs := helper.LoadKeygenSavedData(pIDs)

	for i := 0; i < n; i++ {
		cbs[i] = &MockJobCallback{}
	}

	for i := 0; i < n; i++ {
		p2pCtx := tss.NewPeerContext(pIDs)
		params := tss.NewParameters(p2pCtx, pIDs[i], len(pIDs), threshold)
		jobs[i] = NewPresignJob("Presign0", i, pIDs, params, presignInputs[i], cbs[i], time.Second*15)
	}

	wg := &sync.WaitGroup{}
	wg.Add(n)
	routeJobMesasge(jobs, cbs, wg)

	for _, job := range jobs {
		err := job.Start()
		if err != nil {
			panic(err)
		}
	}

	wg.Wait()

	for i := 0; i < 10; i++ {
		allDone := true
		for _, job := range jobs {
			if !job.isDone() {
				allDone = false
				break
			}
		}
		if allDone {
			break
		}

		time.Sleep(time.Millisecond * 100)
	}

	for _, job := range jobs {
		require.True(t, job.isDone(), "Both endCh and outCh should be done.")
	}
}
