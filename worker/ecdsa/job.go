package ecdsa

import (
	"fmt"
	"math/big"
	"time"

	"github.com/sisu-network/dheart/utils"
	"github.com/sisu-network/dheart/worker/helper"
	libCommon "github.com/sisu-network/tss-lib/common"
	"github.com/sisu-network/tss-lib/ecdsa/keygen"
	"github.com/sisu-network/tss-lib/ecdsa/presign"
	"github.com/sisu-network/tss-lib/ecdsa/signing"
	"github.com/sisu-network/tss-lib/tss"

	wTypes "github.com/sisu-network/dheart/worker/types"
)

type JobCallback interface {
	// Called when there is a tss message output.
	OnJobMessage(job *Job, msg tss.Message)

	// Called when this keygen job finishes.
	OnJobKeygenFinished(job *Job, data *keygen.LocalPartySaveData)

	// Called when this presign job finishes.
	OnJobPresignFinished(job *Job, data *presign.LocalPresignData)

	// Called when this signing job finishes.
	OnJobSignFinished(job *Job, data *libCommon.SignatureData)

	// OnJobTimeout on job timeout
	OnJobTimeout()
}

type Job struct {
	jobType wTypes.WorkType
	index   int

	outCh        chan tss.Message
	endKeygenCh  chan keygen.LocalPartySaveData
	endPresignCh chan presign.LocalPresignData
	endSigningCh chan libCommon.SignatureData
	closeCh      chan struct{}

	party    tss.Party
	callback JobCallback

	timeOut time.Duration
}

func NewKeygenJob(index int, pIDs tss.SortedPartyIDs, params *tss.Parameters, localPreparams *keygen.LocalPreParams, callback JobCallback) *Job {
	outCh := make(chan tss.Message, len(pIDs))
	endCh := make(chan keygen.LocalPartySaveData, len(pIDs))
	closeCh := make(chan struct{}, 1)

	party := keygen.NewLocalParty(params, outCh, endCh, *localPreparams).(*keygen.LocalParty)

	return &Job{
		index:       index,
		jobType:     wTypes.ECDSA_KEYGEN,
		party:       party,
		outCh:       outCh,
		endKeygenCh: endCh,
		callback:    callback,
		closeCh:     closeCh,
		timeOut:     10 * time.Minute,
	}
}

func NewPresignJob(index int, pIDs tss.SortedPartyIDs, params *tss.Parameters, savedData *keygen.LocalPartySaveData, callback JobCallback) *Job {
	outCh := make(chan tss.Message, len(pIDs))
	endCh := make(chan presign.LocalPresignData, len(pIDs))

	party := presign.NewLocalParty(pIDs, params, *savedData, outCh, endCh)

	return &Job{
		index:        index,
		jobType:      wTypes.ECDSA_PRESIGN,
		party:        party,
		outCh:        outCh,
		endPresignCh: endCh,
		callback:     callback,
	}
}

func NewSigningJob(index int, pIDs tss.SortedPartyIDs, params *tss.Parameters, msg string, signingInput *presign.LocalPresignData, callback JobCallback) *Job {
	outCh := make(chan tss.Message, len(pIDs))
	endCh := make(chan libCommon.SignatureData, len(pIDs))

	msgInt := new(big.Int).SetBytes([]byte(msg))
	party := signing.NewLocalParty(msgInt, params, signingInput, outCh, endCh)

	return &Job{
		index:        index,
		jobType:      wTypes.ECDSA_SIGNING,
		party:        party,
		outCh:        outCh,
		endSigningCh: endCh,
		callback:     callback,
	}
}

func (job *Job) Start() error {
	if err := job.party.Start(); err != nil {
		return fmt.Errorf("error when starting party %w", err)
	}

	go job.startListening()
	return nil
}

func (job *Job) Stop() {
	job.closeCh <- struct{}{}
}

func (job *Job) startListening() {
	outCh := job.outCh

	endTime := time.Now().Add(job.timeOut)

	// TODO: Add timeout and missing messages.
	for {
		select {
		case <-time.After(endTime.Sub(time.Now())):
			job.callback.OnJobTimeout()
			return

		case <-job.closeCh:
			utils.LogWarn("job closed")
			return

		case msg := <-outCh:
			job.callback.OnJobMessage(job, msg)

		case data := <-job.endKeygenCh:
			job.callback.OnJobKeygenFinished(job, &data)
			return

		case data := <-job.endPresignCh:
			job.callback.OnJobPresignFinished(job, &data)
			return

		case data := <-job.endSigningCh:
			job.callback.OnJobSignFinished(job, &data)
			return
		}
	}
}

func (job *Job) processMessage(msg tss.Message) *tss.Error {
	return helper.SharedPartyUpdater(job.party, msg)
}
