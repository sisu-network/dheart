package ecdsa

import (
	"encoding/json"
	"errors"

	"github.com/sisu-network/dheart/utils"
	"github.com/sisu-network/dheart/worker"
	"github.com/sisu-network/dheart/worker/helper"
	"github.com/sisu-network/dheart/worker/interfaces"
	"github.com/sisu-network/tss-lib/ecdsa/keygen"
	"github.com/sisu-network/tss-lib/tss"

	"github.com/sisu-network/dheart/types/common"
	commonTypes "github.com/sisu-network/dheart/types/common"
	wTypes "github.com/sisu-network/dheart/worker/types"
)

// A callback for the caller to receive updates from this worker. We use callback instead of Go
// channel to avoid creating too many channels.
type WorkerCallback interface {
	onKeygenFinished(data *keygen.LocalPartySaveData)
}

// Implements DefaultWorker interface
type DefaultWorker struct {
	batchSize int
	myPid     *tss.PartyID
	pIDs      tss.SortedPartyIDs
	p2pCtx    *tss.PeerContext
	jobType   wTypes.WorkType

	threshold      int
	jobs           []*Job
	localPreparams *keygen.LocalPreParams
	dispatcher     interfaces.MessageDispatcher
	savedData      *keygen.LocalPartySaveData

	saveData *keygen.LocalPartySaveData
	errCh    chan error

	callback WorkerCallback
}

func NewKeygenWorker(
	batchSize int,
	pIDs tss.SortedPartyIDs,
	myPid *tss.PartyID,
	localPreparams *keygen.LocalPreParams,
	threshold int,
	dispatcher interfaces.MessageDispatcher,
	errCh chan error,
	callback WorkerCallback,
) worker.Worker {
	p2pCtx := tss.NewPeerContext(pIDs)

	return &DefaultWorker{
		jobType:        wTypes.ECDSA_KEYGEN,
		batchSize:      batchSize,
		myPid:          myPid,
		pIDs:           pIDs,
		p2pCtx:         p2pCtx,
		threshold:      threshold,
		localPreparams: localPreparams,
		dispatcher:     dispatcher,
		errCh:          errCh,
		callback:       callback,
		jobs:           make([]*Job, batchSize),
	}
}

func NewPresignWorker(
	batchSize int,
	pIDs tss.SortedPartyIDs,
	myPid *tss.PartyID,
	params *tss.Parameters,
	savedData *keygen.LocalPartySaveData,
	dispatcher interfaces.MessageDispatcher,
	errCh chan error,
	callback WorkerCallback,
) worker.Worker {
	p2pCtx := tss.NewPeerContext(pIDs)

	return &DefaultWorker{
		jobType:    wTypes.ECDSA_PRESIGN,
		batchSize:  batchSize,
		myPid:      myPid,
		pIDs:       pIDs,
		p2pCtx:     p2pCtx,
		threshold:  len(pIDs) - 1,
		dispatcher: dispatcher,
		errCh:      errCh,
		callback:   callback,
		jobs:       make([]*Job, batchSize),
	}
}

func (w *DefaultWorker) Start() error {
	params := tss.NewParameters(w.p2pCtx, w.myPid, len(w.pIDs), w.threshold)

	// Creates all jobs
	for i := range w.jobs {
		switch w.jobType {
		case wTypes.ECDSA_KEYGEN:
			w.jobs[i] = NewKeygenJob(w.pIDs, w.myPid, params, w.localPreparams, w)

		case wTypes.ECDSA_PRESIGN:
		case wTypes.ECDSA_SIGNING:
		}

		w.jobs[i].Start()
	}

	return nil
}

func (w *DefaultWorker) onError(job *Job, err error) {
	// TODO: handle error here.
	w.errCh <- err
}

// Called when there is a new message from tss-lib
func (w *DefaultWorker) onMessage(job *Job, msg tss.Message) {
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

// Process incoming update message.
func (w *DefaultWorker) ProcessNewMessage(tssMsg *commonTypes.TssMessage) error {
	if w.batchSize != len(tssMsg.UpdateMessages) {
		return errors.New("Batch size does not match")
	}

	// Do all message validation first before processing.
	msgs := make([]tss.ParsedMessage, w.batchSize)
	for i := range w.jobs {
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

		msgs[i] = msg
	}

	// Now udpate all messages
	for i, job := range w.jobs {
		job.processMessage(msgs[i])
	}

	return nil
}

// Implements onKeygenFinished of JobCallback. This function is called from a job after key
// generation finishes.
func (w *DefaultWorker) onKeygenFinished(job *Job, data *keygen.LocalPartySaveData) {
	w.saveData = data

	w.callback.onKeygenFinished(data)
}

// Implements GetId() of Worker interface.
func (w *DefaultWorker) GetId() string {
	return w.myPid.Id
}
