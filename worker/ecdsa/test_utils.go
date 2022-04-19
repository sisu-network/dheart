package ecdsa

import (
	"fmt"
	"sync"
	"time"

	"github.com/sisu-network/dheart/types/common"
	"github.com/sisu-network/dheart/worker"
	"github.com/sisu-network/tss-lib/tss"
)

func startAllWorkers(workers []worker.Worker) {
	// Start all workers
	wg := sync.WaitGroup{}
	wg.Add(len(workers))
	for i := 0; i < len(workers); i++ {
		go func(w worker.Worker) {
			wg.Done()
			if err := w.Start(make([]*common.TssMessage, 0)); err != nil {
				panic(err)
			}
		}(workers[i])
	}

	wg.Wait()
}

// debug function to get worker index from its id
func getWorkerIndex(workers []worker.Worker, id string) int {
	for i, w := range workers {
		if w.GetPartyId() == id {
			return i
		}
	}

	return -1
}

func runAllWorkers(workers []worker.Worker, outCh chan *common.TssMessage, done chan bool) {
	for {
		select {
		case <-done:
			return
		case <-time.After(time.Second * 300):
			panic("Test timeout")

		case tssMsg := <-outCh:
			if tssMsg.From == tssMsg.To {
				continue
			}

			isBroadcast := tssMsg.IsBroadcast()
			if isBroadcast {
				for _, w := range workers {
					if w.GetPartyId() == tssMsg.From {
						continue
					}

					fmt.Println("Broadcasting message from ", tssMsg.From)

					processMsgWithPanicOnFail(w, tssMsg)
				}
			} else {
				if tssMsg.From == tssMsg.To {
					panic("A worker cannot send a message to itself")
				}

				for _, w := range workers {
					if w.GetPartyId() == tssMsg.To {
						fmt.Println("Unitcast: ", w.GetPartyId(), tssMsg.To)
						processMsgWithPanicOnFail(w, tssMsg)
						break
					}
				}
			}
		}
	}
}

func processMsgWithPanicOnFail(w worker.Worker, tssMsg *common.TssMessage) {
	go func(w worker.Worker, tssMsg *common.TssMessage) {
		if err := w.ProcessNewMessage(tssMsg); err != nil {
			panic(err)
		}
	}(w, tssMsg)
}

func flattenPidMaps(m map[string]*tss.PartyID) []*tss.PartyID {
	pids := make([]*tss.PartyID, len(m))
	index := 0

	for _, partyId := range m {
		pids[index] = partyId
		index++
	}

	return pids
}
