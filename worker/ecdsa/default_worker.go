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
	OnWorkKeygenFinished(workerId string, data []*keygen.LocalPartySaveData)

	OnWorkPresignFinished(workerId string, data []*presign.LocalPresignData)
}

// Implements DefaultWorker interface
type DefaultWorker struct {
	batchSize int
	myPid     *tss.PartyID
	pIDs      tss.SortedPartyIDs
	p2pCtx    *tss.PeerContext
	jobType   wTypes.WorkType
	callback  WorkerCallback
	errCh     chan error

	threshold      int
	jobs           []*Job
	localPreparams *keygen.LocalPreParams
	dispatcher     interfaces.MessageDispatcher

	presignInput *keygen.LocalPartySaveData // output from keygen. This field is used for presign.

	// A map between of rounds and
	// key: one of the 2 values
	//      - round if a message is broadcast
	//      - round-partyId if a message is unicast
	// value: list of task that completes this message signing.
	jobOutput     map[string][]tss.Message
	jobOutputLock *sync.RWMutex

	// List of final presign output. This array has length of batchSize.
	keygenOutputs  []*keygen.LocalPartySaveData
	presignOutputs []*presign.LocalPresignData
	outputLock     *sync.RWMutex
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
	worker := baseWorker(batchSize, pIDs, myPid, nil, dispatcher, errCh, callback)

	worker.jobType = wTypes.ECDSA_KEYGEN
	worker.localPreparams = localPreparams
	worker.threshold = threshold
	worker.keygenOutputs = make([]*keygen.LocalPartySaveData, batchSize)

	return worker
}

func NewPresignWorker(
	batchSize int,
	pIDs tss.SortedPartyIDs,
	myPid *tss.PartyID,
	params *tss.Parameters,
	keygenOutput *keygen.LocalPartySaveData,
	dispatcher interfaces.MessageDispatcher,
	errCh chan error,
	callback WorkerCallback,
) worker.Worker {
	worker := baseWorker(batchSize, pIDs, myPid, params, dispatcher, errCh, callback)

	worker.jobType = wTypes.ECDSA_PRESIGN
	worker.presignInput = keygenOutput
	worker.presignOutputs = make([]*presign.LocalPresignData, batchSize)

	return worker
}

func baseWorker(batchSize int,
	pIDs tss.SortedPartyIDs,
	myPid *tss.PartyID,
	params *tss.Parameters,
	dispatcher interfaces.MessageDispatcher,
	errCh chan error,
	callback WorkerCallback,
) *DefaultWorker {
	p2pCtx := tss.NewPeerContext(pIDs)

	return &DefaultWorker{
		batchSize:     batchSize,
		myPid:         myPid,
		pIDs:          pIDs,
		p2pCtx:        p2pCtx,
		dispatcher:    dispatcher,
		errCh:         errCh,
		callback:      callback,
		jobs:          make([]*Job, batchSize),
		jobOutput:     make(map[string][]tss.Message),
		jobOutputLock: &sync.RWMutex{},
		outputLock:    &sync.RWMutex{},
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
			w.jobs[i] = NewPresignJob(i, w.pIDs, w.myPid, params, w.presignInput, w)

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

	// Update the list of finished jobs for msgKey
	w.jobOutputLock.Lock()
	list := w.jobOutput[msgKey]
	if list == nil {
		list = make([]tss.Message, w.batchSize)
	}
	list[job.index] = msg
	w.jobOutput[msgKey] = list
	// Count how many job that have completed.
	count := w.getCompletedJobCount(list)
	w.jobOutputLock.Unlock()

	if count == w.batchSize {
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
			go w.dispatcher.BroadcastMessage(w.pIDs, tssMsg)
		} else {
			go w.dispatcher.UnicastMessage(dest[0], tssMsg)
		}
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
		updateMessage := tssMsg.UpdateMessages[i]
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
	w.outputLock.Lock()
	w.keygenOutputs[job.index] = data
	w.outputLock.Unlock()

	// Count the number of finished job.
	w.outputLock.RLock()
	count := 0
	for _, item := range w.keygenOutputs {
		if item != nil {
			count++
		}
	}
	w.outputLock.RUnlock()

	if count == w.batchSize {
		utils.LogVerbose(w.GetId(), "Done!")
		w.callback.OnWorkKeygenFinished(w.GetId(), w.keygenOutputs)
	}
}

func (w *DefaultWorker) OnJobPresignFinished(job *Job, data *presign.LocalPresignData) {
	w.outputLock.Lock()
	w.presignOutputs[job.index] = data
	w.outputLock.Unlock()

	// Count the number of finished job.
	w.outputLock.RLock()
	count := 0
	for _, item := range w.presignOutputs {
		if item != nil {
			count++
		}
	}
	w.outputLock.RUnlock()

	if count == w.batchSize {
		utils.LogVerbose(w.GetId(), "Done!")
		w.callback.OnWorkPresignFinished(w.GetId(), w.presignOutputs)
	}
}

// Implements GetId() of Worker interface.
func (w *DefaultWorker) GetId() string {
	return w.myPid.Id
}
