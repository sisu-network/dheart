package worker

import (
	"crypto/elliptic"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/sisu-network/dheart/core/message"
	"github.com/sisu-network/dheart/worker/helper"
	"github.com/sisu-network/lib/log"
	libCommon "github.com/sisu-network/tss-lib/common"
	eckeygen "github.com/sisu-network/tss-lib/ecdsa/keygen"
	ecpresign "github.com/sisu-network/tss-lib/ecdsa/presign"
	ecsigning "github.com/sisu-network/tss-lib/ecdsa/signing"
	edkeygen "github.com/sisu-network/tss-lib/eddsa/keygen"
	edsigning "github.com/sisu-network/tss-lib/eddsa/signing"
	"github.com/sisu-network/tss-lib/tss"
	"go.uber.org/atomic"

	wTypes "github.com/sisu-network/dheart/worker/types"
)

type JobFailure int

const (
	JobFailureTimeout JobFailure = iota
)

type JobCallback interface {
	// Called when there is a tss message output.
	OnJobMessage(job *Job, msg tss.Message)

	// Called when this job either produces result or timeouts.
	OnJobResult(job *Job, result JobResult)
}

type JobResult struct {
	Success bool
	Failure JobFailure

	EcKeygen  eckeygen.LocalPartySaveData
	EcPresign *ecpresign.LocalPresignData
	EcSigning *libCommon.ECSignature

	EdKeygen  edkeygen.LocalPartySaveData
	EdSigning *edsigning.SignatureData
}

type Job struct {
	// Immutable data
	workId   string
	jobType  wTypes.WorkType
	index    int
	outCh    chan tss.Message
	party    tss.Party
	callback JobCallback
	timeOut  time.Duration

	// Ecdsa
	ecEndKeygenCh  chan eckeygen.LocalPartySaveData
	ecEndPresignCh chan *ecpresign.LocalPresignData
	ecEndSigningCh chan *libCommon.ECSignature

	// Eddsa
	edEndKeygenCh  chan edkeygen.LocalPartySaveData
	edEndSigningCh chan *edsigning.SignatureData

	// Mutable data
	finishedMsgs map[string]bool
	finishLock   *sync.RWMutex
	doneOutCh    *atomic.Bool
	doneEndCh    *atomic.Bool
}

func NewEcKeygenJob(
	workId string,
	index int,
	pIDs tss.SortedPartyIDs,
	params *tss.Parameters,
	localPreparams *eckeygen.LocalPreParams,
	callback JobCallback,
	timeOut time.Duration,
) *Job {
	outCh := make(chan tss.Message, len(pIDs))
	endCh := make(chan eckeygen.LocalPartySaveData, len(pIDs))

	party := eckeygen.NewLocalParty(params, outCh, endCh, *localPreparams)

	job := baseJob(workId, index, party, wTypes.EcdsaKeygen, callback, outCh, timeOut)
	job.ecEndKeygenCh = endCh

	return job
}

func NewEcPresignJob(
	workId string,
	index int,
	pIDs tss.SortedPartyIDs,
	params *tss.Parameters,
	savedData *eckeygen.LocalPartySaveData,
	callback JobCallback,
	timeOut time.Duration,
) *Job {
	outCh := make(chan tss.Message, len(pIDs))
	endCh := make(chan *ecpresign.LocalPresignData, len(pIDs))

	party := ecpresign.NewLocalParty(params, *savedData, outCh, endCh)

	job := baseJob(workId, index, party, wTypes.EcdsaPresign, callback, outCh, timeOut)
	job.ecEndPresignCh = endCh

	return job
}

func NewEcSigningJob(
	workId string,
	index int,
	pIDs tss.SortedPartyIDs,
	params *tss.Parameters,
	msg string,
	signingInput *ecpresign.LocalPresignData,
	callback JobCallback,
	timeOut time.Duration,
) *Job {
	outCh := make(chan tss.Message, len(pIDs))
	endCh := make(chan *libCommon.ECSignature, len(pIDs))

	msgInt := hashToInt([]byte(msg), tss.EC())
	party := ecsigning.NewLocalParty(msgInt, params, *signingInput, outCh, endCh)

	job := baseJob(workId, index, party, wTypes.EcdsaSigning, callback, outCh, timeOut)
	job.ecEndSigningCh = endCh

	return job
}

func NewEdKeygenJob(
	workId string,
	index int,
	pIDs tss.SortedPartyIDs,
	params *tss.Parameters,
	callback JobCallback,
	timeOut time.Duration,
) *Job {

	outCh := make(chan tss.Message, len(pIDs))
	endCh := make(chan edkeygen.LocalPartySaveData, len(pIDs))
	party := edkeygen.NewLocalParty(params, outCh, endCh)

	job := baseJob(workId, index, party, wTypes.EddsaKeygen, callback, outCh, timeOut)
	job.edEndKeygenCh = endCh

	return job
}

func NewEdSigningJob(
	workId string,
	index int,
	pIDs tss.SortedPartyIDs,
	params *tss.Parameters,
	msg []byte,
	signingInput edkeygen.LocalPartySaveData,
	callback JobCallback,
	timeOut time.Duration,
) *Job {
	outCh := make(chan tss.Message, len(pIDs))
	endCh := make(chan *edsigning.SignatureData, len(pIDs))

	party := edsigning.NewLocalParty(new(big.Int).SetBytes(msg), params, signingInput, outCh, endCh)

	job := baseJob(workId, index, party, wTypes.EddsaKeygen, callback, outCh, timeOut)
	job.edEndSigningCh = endCh

	return job
}

func baseJob(
	workId string,
	index int,
	party tss.Party,
	jobType wTypes.WorkType,
	callback JobCallback,
	outCh chan tss.Message,
	timeOut time.Duration,
) *Job {
	return &Job{
		workId:       workId,
		index:        index,
		party:        party,
		jobType:      jobType,
		callback:     callback,
		outCh:        outCh,
		timeOut:      timeOut,
		finishLock:   &sync.RWMutex{},
		finishedMsgs: make(map[string]bool),
		doneOutCh:    atomic.NewBool(false),
		doneEndCh:    atomic.NewBool(false),
	}
}

func hashToInt(hash []byte, c elliptic.Curve) *big.Int {
	orderBits := c.Params().N.BitLen()
	orderBytes := (orderBits + 7) / 8
	if len(hash) > orderBytes {
		hash = hash[:orderBytes]
	}

	ret := new(big.Int).SetBytes(hash)
	excess := len(hash)*8 - orderBits
	if excess > 0 {
		ret.Rsh(ret, uint(excess))
	}
	return ret
}

func (job *Job) Start() error {
	if err := job.party.Start(); err != nil {
		return fmt.Errorf("error when starting party %w", err)
	}

	go job.startListening()
	return nil
}

func (job *Job) startListening() {
	outCh := job.outCh
	endTime := time.Now().Add(job.timeOut)

	// Wait for one of the end channel.
	for {
		select {
		case <-time.After(endTime.Sub(time.Now())):
			if !job.isDone() {
				log.Warn("Job timeout waiting for end channel")
				go job.callback.OnJobResult(job, JobResult{
					Success: false,
					Failure: JobFailureTimeout,
				})
			}

			return

		case msg := <-outCh:
			job.addOutMessage(msg)
			job.callback.OnJobMessage(job, msg)

			if job.isDone() {
				return
			}

		case data := <-job.ecEndKeygenCh:
			job.doneEndCh.Store(true)
			job.callback.OnJobResult(job, JobResult{
				Success:  true,
				EcKeygen: data,
			})

			if job.isDone() {
				return
			}

		case data := <-job.ecEndPresignCh:
			job.doneEndCh.Store(true)
			job.callback.OnJobResult(job, JobResult{
				Success:   true,
				EcPresign: data,
			})

			if job.isDone() {
				return
			}

		case data := <-job.ecEndSigningCh:
			job.doneEndCh.Store(true)
			job.callback.OnJobResult(job, JobResult{
				Success:   true,
				EcSigning: data,
			})

			if job.isDone() {
				return
			}

		case data := <-job.edEndKeygenCh:
			job.doneEndCh.Store(true)
			job.callback.OnJobResult(job, JobResult{
				Success:  true,
				EdKeygen: data,
			})

			if job.isDone() {
				return
			}

		case data := <-job.edEndSigningCh:
			job.doneEndCh.Store(true)
			job.callback.OnJobResult(job, JobResult{
				Success:   true,
				EdSigning: data,
			})

			if job.isDone() {
				return
			}
		}
	}
}

func (job *Job) processMessage(msg tss.Message) *tss.Error {
	err := helper.SharedPartyUpdater(job.party, msg)
	if err != nil {
		log.Error("Failed to process message:", msg.Type(), "err = ", err)
	}
	return err
}

func (job *Job) addOutMessage(msg tss.Message) {
	job.finishLock.Lock()
	job.finishedMsgs[msg.Type()] = true
	count := len(job.finishedMsgs)
	job.finishLock.Unlock()

	if count == message.GetMessageCountByWorkType(job.jobType) {
		// Mark the outCh done
		job.doneOutCh.Store(true)
	}
}

func (job *Job) isDone() bool {
	return job.doneEndCh.Load() && job.doneOutCh.Load()
}
