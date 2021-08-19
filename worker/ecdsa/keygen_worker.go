package ecdsa

import (
	"encoding/json"
	"errors"

	"github.com/sisu-network/dheart/utils"
	"github.com/sisu-network/dheart/worker/helper"
	"github.com/sisu-network/dheart/worker/interfaces"
	"github.com/sisu-network/tss-lib/ecdsa/keygen"
	"github.com/sisu-network/tss-lib/tss"

	"github.com/sisu-network/dheart/types/common"
	commonTypes "github.com/sisu-network/dheart/types/common"
)

type KeygenWorkerCallback interface {
	onKeygenFinished(data *keygen.LocalPartySaveData)
}

// Implements Worker interface
type KeygenWorker struct {
	myPid  *tss.PartyID
	pIDs   tss.SortedPartyIDs
	p2pCtx *tss.PeerContext

	threshold      int
	job            *KeygenJob
	localPreparams *keygen.LocalPreParams
	dispatcher     interfaces.MessageDispatcher

	saveData *keygen.LocalPartySaveData
	errCh    chan error

	callback KeygenWorkerCallback
}

func NewKeygenWorker(pIDs tss.SortedPartyIDs,
	myPid *tss.PartyID,
	localPreparams *keygen.LocalPreParams,
	threshold int,
	dispatcher interfaces.MessageDispatcher,
	errCh chan error,
	callback KeygenWorkerCallback,
) *KeygenWorker {
	p2pCtx := tss.NewPeerContext(pIDs)

	return &KeygenWorker{
		myPid:          myPid,
		pIDs:           pIDs,
		p2pCtx:         p2pCtx,
		threshold:      threshold,
		localPreparams: localPreparams,
		dispatcher:     dispatcher,
		errCh:          errCh,
		callback:       callback,
	}
}

func (w *KeygenWorker) Start() error {
	// Creates a single job
	params := tss.NewParameters(w.p2pCtx, w.myPid, len(w.pIDs), w.threshold)

	w.job = NewJob(w.pIDs, w.myPid, params, w.localPreparams, w)
	w.job.Start()

	return nil
}

func (w *KeygenWorker) onError(job *KeygenJob, err error) {
	// TODO: handle error here.
	w.errCh <- err
}

// Called when there is a new message from tss-lib
func (w *KeygenWorker) onMessage(job *KeygenJob, msg tss.Message) {
	dest := msg.GetTo()
	to := ""
	if dest != nil {
		to = dest[0].Id
	}

	tssMsg, err := common.NewTssMessage(w.myPid.Id, to, []tss.Message{msg}, msg.Type())
	if err != nil {
		utils.LogCritical("Cannot build TSS message, err =", err)
	}

	if dest == nil {
		// broadcast
		w.dispatcher.BroadcastMessage(w.pIDs, tssMsg)
	} else {
		w.dispatcher.UnicastMessage(dest[0], tssMsg)
	}
}

func (w *KeygenWorker) processNewMessage(tssMsg *commonTypes.TssMessage) error {
	fromString := tssMsg.From
	from := helper.GetFromPid(fromString, w.pIDs)
	if from == nil {
		return errors.New("Sender is nil")
	}

	// TODO: Check message size here.
	updateMessage := tssMsg.UpdateMessages[0]
	msg, err := tss.ParseWireMessage(updateMessage.Data, from, tssMsg.IsBroadcast())
	if err != nil {
		utils.LogError(err)
		return err
	}

	msgRouting := tss.MessageRouting{}
	err = json.Unmarshal(updateMessage.SerializedMessageRouting, &msgRouting)
	if err != nil {
		utils.LogError(err)
		return err
	}

	w.job.processMessage(msg)

	return nil
}

func (w *KeygenWorker) onFinished(job *KeygenJob, data *keygen.LocalPartySaveData) {
	w.saveData = data

	w.callback.onKeygenFinished(data)
}
