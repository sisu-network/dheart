package ecdsa

import (
	"crypto/elliptic"
	"fmt"
	"github.com/sisu-network/dheart/core/message"
	"math/big"
	"time"

	"github.com/sisu-network/dheart/worker/helper"
	"github.com/sisu-network/lib/log"
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

	// OnRequestTSSMessageFromPeers potentially missed message to process current round
	// we need to request message for this round from peers
	OnRequestTSSMessageFromPeers(job *Job, msgKey string, pIDs []*tss.PartyID)

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

	getCurRoundFunc func() uint32
}

func NewKeygenJob(
	index int,
	pIDs tss.SortedPartyIDs,
	params *tss.Parameters,
	localPreparams *keygen.LocalPreParams,
	callback JobCallback,
	timeOut time.Duration,
	getCurRoundFunc func() uint32,
) *Job {
	outCh := make(chan tss.Message, len(pIDs))
	endCh := make(chan keygen.LocalPartySaveData, len(pIDs))
	closeCh := make(chan struct{}, 1)

	party := keygen.NewLocalParty(params, outCh, endCh, *localPreparams).(*keygen.LocalParty)

	return &Job{
		index:           index,
		jobType:         wTypes.EcdsaKeygen,
		party:           party,
		outCh:           outCh,
		endKeygenCh:     endCh,
		callback:        callback,
		closeCh:         closeCh,
		timeOut:         timeOut,
		getCurRoundFunc: getCurRoundFunc,
	}
}

func NewPresignJob(
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

func (job *Job) Stop() {
	job.closeCh <- struct{}{}
}

func (job *Job) startListening() {
	outCh := job.outCh

	endTime := time.Now().Add(job.timeOut)

	// TODO: Add timeout and missing messages.
	ticker := time.NewTicker(10 * time.Second)
	oldRound := job.getCurRoundFunc()
	for {
		select {
		case <-ticker.C:
			// Every 10 seconds, check if job round is changed?
			// If not, potentially we missed some messages from peers
			// In this case, send request to ask broadcast/unicast message from peers and re-process this round

			// Check round number has changed or not
			currentRound := job.getCurRoundFunc()
			if currentRound != oldRound {
				// Re-assign old round
				oldRound = currentRound
				continue
			}

			waitingForParties := job.party.WaitingFor()
			msgTypes := message.GetAllMessageTypesByRound(currentRound)
			for _, msgType := range msgTypes {
				requestMsgKey := GetCacheMsgKey(msgType, job.party.PartyID().GetId(), "")
				job.callback.OnRequestTSSMessageFromPeers(job, requestMsgKey, waitingForParties)
			}
		case <-time.After(endTime.Sub(time.Now())):
			job.callback.OnJobTimeout()
			return

		case <-job.closeCh:
			log.Warn("job closed")
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
