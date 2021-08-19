package worker

import "github.com/sisu-network/tss-lib/tss"

// Default implementation of MessageDispatcher that sends messages through network.
type NetworkDispatcher struct {
}

func NewNetworkDispatcher() *NetworkDispatcher {
	return &NetworkDispatcher{}
}

func (d *NetworkDispatcher) BroadcastMessage(pIDs tss.SortedPartyIDs, msg tss.Message) {
}

func (d *NetworkDispatcher) UnicastMessage(msg tss.Message) {
}
