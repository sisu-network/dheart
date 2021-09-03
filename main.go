package main

import (
	"math/big"
	"os/signal"
	"syscall"

	"os"

	"github.com/sisu-network/tss-lib/tss"

	"github.com/sisu-network/dheart/p2p"
	"github.com/sisu-network/dheart/run"
)

func getSortedPartyIds(n int) tss.SortedPartyIDs {
	keys := p2p.GetAllPrivateKeys(n)
	partyIds := make([]*tss.PartyID, n)

	// Creates list of party ids
	for i := 0; i < n; i++ {
		bz := keys[i].PubKey().Bytes()
		peerId := p2p.P2PIDFromKey(keys[i])
		party := tss.NewPartyID(peerId.String(), "", new(big.Int).SetBytes(bz))
		partyIds[i] = party
	}

	return tss.SortPartyIDs(partyIds, 0)
}

func main() {
	run.Run()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
}
