package ecdsa

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/sisu-network/dheart/types/common"
	"github.com/sisu-network/dheart/worker"
)

//---/

func startAllWorkers(workers []worker.Worker) {
	// Start all workers
	wg := sync.WaitGroup{}
	wg.Add(len(workers))
	for i := 0; i < len(workers); i++ {
		go func(w worker.Worker) {
			if err := w.Start(make([]*common.TssMessage, 0)); err != nil {
				panic(err)
			}
			wg.Done()
		}(workers[i])
	}

	wg.Wait()
}

func runAllWorkers(workers []worker.Worker, outCh chan *common.TssMessage, errCh chan error, done chan bool) {
	indexMap := make(map[string]int)
	for i := range workers {
		indexMap[workers[i].GetPartyId()] = i
	}

	for {
		select {
		case err := <-errCh:
			panic(err)
		case <-done:
			return
		case <-time.After(time.Second * 300):
			panic(errors.New("Test timeout"))

		case tssMsg := <-outCh:
			isBroadcast := tssMsg.IsBroadcast()
			fmt.Println("Message from -> to:", indexMap[tssMsg.From], indexMap[tssMsg.To], tssMsg.UpdateMessages[0].Round)

			if isBroadcast {
				for _, w := range workers {
					if w.GetPartyId() == tssMsg.From {
						continue
					}

					processMsgWithPanicOnFail(w, tssMsg)
				}
			} else {
				if tssMsg.From == tssMsg.To {
					panic("A worker cannot send a message to itself")
				}

				for _, w := range workers {
					if w.GetPartyId() == tssMsg.To {
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
		err := w.ProcessNewMessage(tssMsg)
		if err != nil {
			panic(err)
		}
	}(w, tssMsg)
}
