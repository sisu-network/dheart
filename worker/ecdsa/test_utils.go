package ecdsa

import (
	"github.com/sisu-network/dheart/types/common"
	"github.com/sisu-network/tss-lib/ecdsa/keygen"
	"github.com/sisu-network/tss-lib/tss"
)

type TestDispatcher struct {
	msgCh chan *common.TssMessage
}

func NewTestDispatcher(msgCh chan *common.TssMessage) *TestDispatcher {
	return &TestDispatcher{
		msgCh: msgCh,
	}
}

// Send a message to a single destination.

func (d *TestDispatcher) BroadcastMessage(pIDs []*tss.PartyID, tssMessage *common.TssMessage) {
	d.msgCh <- tssMessage
}

func (d *TestDispatcher) UnicastMessage(dest *tss.PartyID, tssMessage *common.TssMessage) {
	d.msgCh <- tssMessage
}

//---/

type TestKeygenCallback struct {
	internalCb func(data *keygen.LocalPartySaveData)
}

func NewTestKeygenCallback(internalCb func(data *keygen.LocalPartySaveData)) *TestKeygenCallback {
	return &TestKeygenCallback{
		internalCb: internalCb,
	}
}

func (cb *TestKeygenCallback) onKeygenFinished(data *keygen.LocalPartySaveData) {
	cb.internalCb(data)
}
