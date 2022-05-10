package ecdsa

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
	"github.com/sisu-network/tss-lib/ecdsa/keygen"
	"github.com/sisu-network/tss-lib/ecdsa/presign"
	"github.com/sisu-network/tss-lib/ecdsa/signing"
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

	KeygenData  keygen.LocalPartySaveData
	PresignData *presign.LocalPresignData
	SigningData *libCommon.ECSignature
}

type Job struct {
	// Immutable data
	workId       string
	jobType      wTypes.WorkType
	index        int
	outCh        chan tss.Message
	endKeygenCh  chan keygen.LocalPartySaveData
	endPresignCh chan *presign.LocalPresignData
	endSigningCh chan *libCommon.ECSignature
	party        tss.Party
	callback     JobCallback
	timeOut      time.Duration

	// Mutable data
	finishedMsgs map[string]bool
	finishLock   *sync.RWMutex
	doneOutCh    *atomic.Bool
	doneEndCh    *atomic.Bool
}

func NewKeygenJob(
	workId string,
	index int,
	pIDs tss.SortedPartyIDs,
	params *tss.Parameters,
	localPreparams *keygen.LocalPreParams,
	callback JobCallback,
	timeOut time.Duration,
) *Job {
	outCh := make(chan tss.Message, len(pIDs))
	endCh := make(chan keygen.LocalPartySaveData, len(pIDs))

	party := keygen.NewLocalParty(params, outCh, endCh, *localPreparams).(*keygen.LocalParty)

	job := baseJob(workId, index, party, wTypes.EcdsaKeygen, callback, timeOut)
	job.outCh = outCh
	job.endKeygenCh = endCh

	return job
}

func NewPresignJob(
	workId string,
	index int,
	pIDs tss.SortedPartyIDs,
	params *tss.Parameters,
	savedData *keygen.LocalPartySaveData,
	callback JobCallback,
	timeOut time.Duration,
) *Job {
	outCh := make(chan tss.Message, len(pIDs))
	endCh := make(chan *presign.LocalPresignData, len(pIDs))

	party := presign.NewLocalParty(params, *savedData, outCh, endCh)

	job := baseJob(workId, index, party, wTypes.EcdsaPresign, callback, timeOut)
	job.outCh = outCh
	job.endPresignCh = endCh

	return job
}

func NewSigningJob(
	workId string,
	index int,
	pIDs tss.SortedPartyIDs,
	params *tss.Parameters,
	msg string,
	signingInput *presign.LocalPresignData,
	callback JobCallback,
	timeOut time.Duration,
) *Job {
	outCh := make(chan tss.Message, len(pIDs))
	endCh := make(chan *libCommon.ECSignature, len(pIDs))

	msgInt := hashToInt([]byte(msg), tss.EC())
	party := signing.NewLocalParty(msgInt, params, *signingInput, outCh, endCh)

	job := baseJob(workId, index, party, wTypes.EcdsaSigning, callback, timeOut)

	job.outCh = outCh
	job.endSigningCh = endCh

	return job
}

func baseJob(
	workId string,
	index int,
	party tss.Party,
	jobType wTypes.WorkType,
	callback JobCallback,
	timeOut time.Duration,
) *Job {
	return &Job{
		workId:       workId,
		index:        index,
		party:        party,
		jobType:      jobType,
		callback:     callback,
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

		case data := <-job.endKeygenCh:
			job.doneEndCh.Store(true)
			job.callback.OnJobResult(job, JobResult{
				Success:    true,
				KeygenData: data,
			})

			if job.isDone() {
				return
			}

		case data := <-job.endPresignCh:
			job.doneEndCh.Store(true)
			job.callback.OnJobResult(job, JobResult{
				Success:     true,
				PresignData: data,
			})

			if job.isDone() {
				return
			}

		case data := <-job.endSigningCh:
			job.doneEndCh.Store(true)
			job.callback.OnJobResult(job, JobResult{
				Success:     true,
				SigningData: data,
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
