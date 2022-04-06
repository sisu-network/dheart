package helper

import (
	"github.com/sisu-network/lib/log"
	"github.com/sisu-network/tss-lib/tss"
)

func SharedPartyUpdater(party tss.Party, msg tss.Message) *tss.Error {
	// Do not send a message from this party back to itself
	if party.PartyID() == msg.GetFrom() {
		return nil
	}

	bz, _, err := msg.WireBytes()
	if err != nil {
		log.Error("error when start party updater", err)
		return party.WrapError(err)
	}

	pMsg, err := tss.ParseWireMessage(bz, msg.GetFrom(), msg.IsBroadcast())
	if err != nil {
		log.Error("error when start party updater", err)
		return party.WrapError(err)
	}

	if _, err := party.Update(pMsg); err != nil {
		log.Error("error when start party updater", err)
		return party.WrapError(err)
	}

	return nil
}
