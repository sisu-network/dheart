package ecdsa

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/sisu-network/dheart/utils"
	"github.com/sisu-network/dheart/worker"
	"github.com/sisu-network/dheart/worker/helper"
	"github.com/sisu-network/dheart/worker/interfaces"
	"github.com/sisu-network/tss-lib/ecdsa/keygen"
	"github.com/sisu-network/tss-lib/ecdsa/presign"
	"github.com/sisu-network/tss-lib/tss"

	"github.com/sisu-network/dheart/types/common"
	commonTypes "github.com/sisu-network/dheart/types/common"
	wTypes "github.com/sisu-network/dheart/worker/types"
)

// A callback for the caller to receive updates from this worker. We use callback instead of Go
// channel to avoid creating too many channels.
type WorkerCallback interface {
	OnWorkKeygenFinished(workerId string, data *keygen.LocalPartySaveData)

	OnWorkPresignFinished(workerId string, data *presign.LocalPresignData)
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

	keygenOutputs []*keygen.LocalPartySaveData // output from keygen. This field is used for presign.
	// A map between of rounds and
	// key: one of the 2 values
	//      - round if a message is broadcast
	//      - round-partyId if a message is unicast
	// value: list of task that completes this message signing.
	jobOutput *sync.Map
	errCh     chan error

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
		jobOutput:      &sync.Map{},
	}
}

func NewPresignWorker(
	batchSize int,
	pIDs tss.SortedPartyIDs,
	myPid *tss.PartyID,
	params *tss.Parameters,
	keygenOutputs []*keygen.LocalPartySaveData,
	dispatcher interfaces.MessageDispatcher,
	errCh chan error,
	callback WorkerCallback,
) worker.Worker {
	p2pCtx := tss.NewPeerContext(pIDs)

	return &DefaultWorker{
		jobType:       wTypes.ECDSA_PRESIGN,
		batchSize:     batchSize,
		myPid:         myPid,
		pIDs:          pIDs,
		p2pCtx:        p2pCtx,
		threshold:     len(pIDs) - 1,
		dispatcher:    dispatcher,
		keygenOutputs: keygenOutputs,
		errCh:         errCh,
		callback:      callback,
		jobs:          make([]*Job, batchSize),
		jobOutput:     &sync.Map{},
	}
}

func (w *DefaultWorker) Start() error {
	params := tss.NewParameters(w.p2pCtx, w.myPid, len(w.pIDs), w.threshold)

	// Creates all jobs
	for i := range w.jobs {
		switch w.jobType {
		case wTypes.ECDSA_KEYGEN:
			w.jobs[i] = NewKeygenJob(i, w.pIDs, w.myPid, params, w.localPreparams, w)

		case wTypes.ECDSA_PRESIGN:
			w.jobs[i] = NewPresignJob(i, w.pIDs, w.myPid, params, w.keygenOutputs[i], w)

		case wTypes.ECDSA_SIGNING:

		default:
			return errors.New(fmt.Sprint("Unknown job type", w.jobType))
		}
	}

	// Start all job
	for _, job := range w.jobs {
		job.Start()
	}

	return nil
}

func (w *DefaultWorker) onError(job *Job, err error) {
	// TODO: handle error here.
	w.errCh <- err
}

// Called when there is a new message from tss-lib
func (w *DefaultWorker) OnJobMessage(job *Job, msg tss.Message) {
	// Update the list of completed jobs for current round (in the message)
	msgKey := msg.Type()
	if !msg.IsBroadcast() {
		msgKey = msgKey + "-" + msg.GetTo()[0].Id
	}

	value, ok := w.jobOutput.Load(msgKey)
	if !ok {
		value = make([]tss.Message, w.batchSize)
	}
	list := value.([]tss.Message)
	list[job.index] = msg
	w.jobOutput.Store(msgKey, list)

	count := w.getCompletedJobCount(list)
	if count == w.batchSize {
		fmt.Println("All jobs finished for round ", msgKey)

		// We have completed all job for current round. Send the list to the dispatcher.
		dest := msg.GetTo()
		to := ""
		if dest != nil {
			to = dest[0].Id
		}

		tssMsg, err := common.NewTssMessage(w.myPid.Id, to, list, msg.Type())
		if err != nil {
			utils.LogCritical("Cannot build TSS message, err =", err)
			return
		}

		if dest == nil {
			// broadcast
			w.dispatcher.BroadcastMessage(w.pIDs, tssMsg)
		} else {
			w.dispatcher.UnicastMessage(dest[0], tssMsg)
		}

		// Delete the list from the output map.
		// w.jobOutput.Delete(msgKey)
	}
}

func (w *DefaultWorker) getCompletedJobCount(list []tss.Message) int {
	count := 0
	for _, item := range list {
		if item != nil {
			count++
		}
	}

	return count
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
		go job.processMessage(msgs[i])
	}

	return nil
}

// Implements OnJobKeygenFinished of JobCallback. This function is called from a job after key
// generation finishes.
func (w *DefaultWorker) OnJobKeygenFinished(job *Job, data *keygen.LocalPartySaveData) {
	w.callback.OnWorkKeygenFinished(w.GetId(), data)
}

func (w *DefaultWorker) OnJobPresignFinished(job *Job, data *presign.LocalPresignData) {
	w.callback.OnWorkPresignFinished(w.GetId(), data)
}

// Implements GetId() of Worker interface.
func (w *DefaultWorker) GetId() string {
	return w.myPid.Id
}
