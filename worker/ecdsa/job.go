package ecdsa

import (
	"crypto/elliptic"
	"fmt"
	"math/big"
	"time"

	"github.com/sisu-network/dheart/worker/helper"
	libCommon "github.com/sisu-network/tss-lib/common"
	"github.com/sisu-network/tss-lib/ecdsa/keygen"
	"github.com/sisu-network/tss-lib/ecdsa/presign"
	"github.com/sisu-network/tss-lib/ecdsa/signing"
	"github.com/sisu-network/tss-lib/tss"

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
	PresignData presign.LocalPresignData
	SigningData libCommon.SignatureData
}

type Job struct {
	workId  string
	jobType wTypes.WorkType
	index   int

	outCh        chan tss.Message
	endKeygenCh  chan keygen.LocalPartySaveData
	endPresignCh chan presign.LocalPresignData
	endSigningCh chan libCommon.SignatureData

	party    tss.Party
	callback JobCallback

	timeOut time.Duration
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

	return &Job{
		workId:      workId,
		index:       index,
		jobType:     wTypes.EcdsaKeygen,
		party:       party,
		outCh:       outCh,
		endKeygenCh: endCh,
		callback:    callback,
		timeOut:     timeOut,
	}
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
	endCh := make(chan presign.LocalPresignData, len(pIDs))

	party := presign.NewLocalParty(pIDs, params, *savedData, outCh, endCh)

	return &Job{
		workId:       workId,
		index:        index,
		jobType:      wTypes.EcdsaPresign,
		party:        party,
		outCh:        outCh,
		endPresignCh: endCh,
		callback:     callback,
		timeOut:      timeOut,
	}
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
	endCh := make(chan libCommon.SignatureData, len(pIDs))

	msgInt := hashToInt([]byte(msg), tss.EC())
	party := signing.NewLocalParty(msgInt, params, signingInput, outCh, endCh)

	return &Job{
		workId:       workId,
		index:        index,
		jobType:      wTypes.EcdsaSigning,
		party:        party,
		outCh:        outCh,
		endSigningCh: endCh,
		callback:     callback,
		timeOut:      timeOut,
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

	for {
		select {
		case <-time.After(endTime.Sub(time.Now())):
			job.callback.OnJobResult(job, JobResult{
				Success: false,
				Failure: JobFailureTimeout,
			})
			return

		case msg := <-outCh:
			job.callback.OnJobMessage(job, msg)

		case data := <-job.endKeygenCh:
			fmt.Println("Output is produced")
			job.callback.OnJobResult(job, JobResult{
				Success:    true,
				KeygenData: data,
			})
			return

		case data := <-job.endPresignCh:
			job.callback.OnJobResult(job, JobResult{
				Success:     true,
				PresignData: data,
			})
			return

		case data := <-job.endSigningCh:
			job.callback.OnJobResult(job, JobResult{
				Success:     true,
				SigningData: data,
			})
			return
		}
	}
}

func (job *Job) processMessage(msg tss.Message) *tss.Error {
	return helper.SharedPartyUpdater(job.party, msg)
}
