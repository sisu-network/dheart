package ecdsa

import (
	"github.com/sisu-network/dheart/utils"
	"github.com/sisu-network/dheart/worker/helper"
	libCommon "github.com/sisu-network/tss-lib/common"
	"github.com/sisu-network/tss-lib/ecdsa/keygen"
	"github.com/sisu-network/tss-lib/ecdsa/presign"
	"github.com/sisu-network/tss-lib/tss"

	wTypes "github.com/sisu-network/dheart/worker/types"
)

type JobCallback interface {
	onError(job *Job, err error)

	OnJobMessage(job *Job, msg tss.Message)

	// Called when this keygen job finishes.
	OnJobKeygenFinished(job *Job, data *keygen.LocalPartySaveData)

	// Called when this presign job finishes.
	OnJobPresignFinished(job *Job, data *presign.LocalPresignData)
}

type Job struct {
	jobType wTypes.WorkType
	index   int

	outCh        chan tss.Message
	endKeygenCh  chan keygen.LocalPartySaveData
	endPresignCh chan presign.LocalPresignData
	endSigningCh chan libCommon.SignatureData
	errCh        chan *tss.Error

	party    tss.Party
	callback JobCallback
}

func NewKeygenJob(index int, pIDs tss.SortedPartyIDs, myPid *tss.PartyID, params *tss.Parameters, localPreparams *keygen.LocalPreParams, callback JobCallback) *Job {
	errCh := make(chan *tss.Error, len(pIDs))
	outCh := make(chan tss.Message, len(pIDs))
	endCh := make(chan keygen.LocalPartySaveData, len(pIDs))

	party := keygen.NewLocalParty(params, outCh, endCh, *localPreparams).(*keygen.LocalParty)

	return &Job{
		index:       index,
		jobType:     wTypes.ECDSA_KEYGEN,
		party:       party,
		errCh:       errCh,
		outCh:       outCh,
		endKeygenCh: endCh,
		callback:    callback,
	}
}

func NewPresignJob(index int, pIDs tss.SortedPartyIDs, myPid *tss.PartyID, params *tss.Parameters, savedData *keygen.LocalPartySaveData, callback JobCallback) *Job {
	errCh := make(chan *tss.Error, len(pIDs))
	outCh := make(chan tss.Message, len(pIDs))
	endCh := make(chan presign.LocalPresignData, len(pIDs))

	party := presign.NewLocalParty(pIDs, params, *savedData, outCh, endCh)

	return &Job{
		index:        index,
		jobType:      wTypes.ECDSA_PRESIGN,
		party:        party,
		errCh:        errCh,
		outCh:        outCh,
		endPresignCh: endCh,
		callback:     callback,
	}
}

func (job *Job) Start() {
	if err := job.party.Start(); err != nil {
		utils.LogError("Cannot start a keygen job, err =", err)

		job.errCh <- err
		return
	}

	go job.startListening()
}

func (job *Job) startListening() {
	errCh := job.errCh
	outCh := job.outCh

	// TODO: Add timeout and missing messages.
	for {
		select {
		case err := <-errCh:
			job.onError(err)
			return

		case msg := <-outCh:
			job.callback.OnJobMessage(job, msg)

		case data := <-job.endKeygenCh:
			job.callback.OnJobKeygenFinished(job, &data)
			return

		case data := <-job.endPresignCh:
			job.callback.OnJobPresignFinished(job, &data)
			return
		}
	}
}

func (job *Job) processMessage(msg tss.Message) {
	helper.SharedPartyUpdater(job.party, msg, job.errCh)
}

func (job *Job) onError(err error) {
	utils.LogError(err)
}
