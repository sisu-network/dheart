package core

import (
	"sync"

	libCommon "github.com/sisu-network/tss-lib/common"

	"github.com/sisu-network/dheart/utils"
	"github.com/sisu-network/dheart/worker"
	"github.com/sisu-network/dheart/worker/ecdsa"
	"github.com/sisu-network/dheart/worker/interfaces"
	"github.com/sisu-network/dheart/worker/types"
	"github.com/sisu-network/tss-lib/ecdsa/keygen"
	"github.com/sisu-network/tss-lib/ecdsa/presign"
	"github.com/sisu-network/tss-lib/tss"
)

const (
	MAX_WORKER = 2
	BATCH_SIZE = 4
)

type EngineCallback interface {
	OnWorkKeygenFinished(workerId string, data []*keygen.LocalPartySaveData)

	OnWorkPresignFinished(workerId string, data []*presign.LocalPresignData)

	OnWorkSigningFinished(workerId string, data []*libCommon.SignatureData)
}

// An Engine is a main component for TSS signing. It takes the following roles:
// - Keep track of a list of running workers.
// - Cache message sent to a worker even before the worker starts. Please note that workers in the
//      networking performing the same work might not start at the same time.
// - Route a message to appropriate worker.
type Engine struct {
	myPid *tss.PartyID

	workers      map[string]worker.Worker
	requestQueue *requestQueue
	dispatcher   interfaces.MessageDispatcher

	workLock     *sync.RWMutex
	preworkCache *PreworkMessageCache

	callback EngineCallback
}

func NewEngine(myPid *tss.PartyID, dispatcher interfaces.MessageDispatcher, callback EngineCallback) *Engine {
	return &Engine{
		myPid:        myPid,
		dispatcher:   dispatcher,
		workers:      make(map[string]worker.Worker),
		requestQueue: NewRequestQueue(),
		workLock:     &sync.RWMutex{},
		preworkCache: NewPreworkMessageCache(),
		callback:     callback,
	}
}

func (engine *Engine) AddRequest(request *types.WorkRequest) {
	if err := request.Validate(); err != nil {
		utils.LogError(err)
		return
	}

	engine.workLock.Lock()
	defer engine.workLock.Unlock()

	if engine.requestQueue.AddWork(request) && len(engine.workers) < MAX_WORKER {
		work := engine.requestQueue.Pop()
		go engine.startWork(work)
	}
}

func (engine *Engine) startWork(request *types.WorkRequest) {
	var w worker.Worker
	errCh := make(chan error)
	// Create a new worker.
	switch request.WorkType {
	case types.ECDSA_KEYGEN:
		w = ecdsa.NewKeygenWorker(request.WorkId, BATCH_SIZE, request.PIDs, engine.myPid,
			request.KeygenInput, request.Threshold, engine.dispatcher, errCh, engine)

	case types.ECDSA_PRESIGN:
		p2pCtx := tss.NewPeerContext(request.PIDs)
		params := tss.NewParameters(p2pCtx, engine.myPid, len(request.PIDs), len(request.PIDs)-1)

		w = ecdsa.NewPresignWorker(request.WorkId, BATCH_SIZE, request.PIDs, engine.myPid,
			params, request.PresignInput, engine.dispatcher, errCh, engine)

	case types.ECDSA_SIGNING:
	}

	engine.workLock.Lock()
	engine.workers[request.WorkId] = w
	engine.workLock.Unlock()

	cachedMsgs := engine.preworkCache.PopAllMessages(request.WorkId)

	w.Start(cachedMsgs)
}

func (engine *Engine) OnWorkKeygenFinished(workerId string, data []*keygen.LocalPartySaveData) {
	// TODO: save output.
	engine.callback.OnWorkKeygenFinished(workerId, data)
}

func (engine *Engine) OnWorkPresignFinished(workerId string, data []*presign.LocalPresignData) {
	// TODO: save output.
	engine.callback.OnWorkPresignFinished(workerId, data)
}

func (engine *Engine) OnWorkSigningFinished(workerId string, data []*libCommon.SignatureData) {
	// TODO: save output.
	engine.callback.OnWorkSigningFinished(workerId, data)
}
