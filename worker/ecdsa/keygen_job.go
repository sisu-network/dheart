package ecdsa

import (
	"github.com/sisu-network/dheart/utils"
	"github.com/sisu-network/dheart/worker/helper"
	"github.com/sisu-network/tss-lib/ecdsa/keygen"
	"github.com/sisu-network/tss-lib/tss"
)

type KeygenCallback interface {
	onError(job *KeygenJob, err error)
	onMessage(job *KeygenJob, msg tss.Message)
	onFinished(job *KeygenJob, data *keygen.LocalPartySaveData)
}

type KeygenJob struct {
	index int
	outCh chan tss.Message
	endCh chan keygen.LocalPartySaveData
	errCh chan *tss.Error

	party    tss.Party
	callback KeygenCallback
}

func NewJob(pIDs tss.SortedPartyIDs, myPid *tss.PartyID, params *tss.Parameters, localPreparams *keygen.LocalPreParams, callback KeygenCallback) *KeygenJob {
	errCh := make(chan *tss.Error, len(pIDs))
	outCh := make(chan tss.Message, len(pIDs))
	endCh := make(chan keygen.LocalPartySaveData, len(pIDs))

	party := keygen.NewLocalParty(params, outCh, endCh, *localPreparams).(*keygen.LocalParty)

	return &KeygenJob{
		party:    party,
		errCh:    errCh,
		outCh:    outCh,
		endCh:    endCh,
		callback: callback,
	}
}

func (job *KeygenJob) Start() {
	if err := job.party.Start(); err != nil {
		utils.LogError("Cannot start a keygen job, err =", err)

		job.errCh <- err
		return
	}

	go job.startListening()
}

func (job *KeygenJob) startListening() {
	errCh := job.errCh
	outCh := job.outCh
	endCh := job.endCh

	for {
		select {
		case err := <-errCh:
			job.onError(err)
			return

		case msg := <-outCh:
			job.callback.onMessage(job, msg)

		case data := <-endCh:
			job.callback.onFinished(job, &data)
			return
		}
	}
}

func (job *KeygenJob) processMessage(msg tss.Message) {
	helper.SharedPartyUpdater(job.party, msg, job.errCh)
}

func (job *KeygenJob) onError(err error) {
	utils.LogError(err)
}
