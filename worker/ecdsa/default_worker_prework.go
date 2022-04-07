package ecdsa

import (
	"errors"
	"fmt"
	"time"

	"github.com/sisu-network/dheart/types/common"
	commonTypes "github.com/sisu-network/dheart/types/common"
	"github.com/sisu-network/dheart/worker"
	"github.com/sisu-network/dheart/worker/helper"
	"github.com/sisu-network/dheart/worker/types"
	"github.com/sisu-network/lib/log"
	"github.com/sisu-network/tss-lib/tss"
)

// preExecution finds list of nodes who are available for doing this TSS task.
// Step1: Find a leader using the hash of workId and list of all party ids. This leader should be
//      the same among all parties.
// Step2:
//     - If this node is a leader, broadcast availableRequest to everyone. Gather all response
//         and sends the results to everyone
//     - If this node is a member, send "YES" to the leader.
func (w *DefaultWorker) preExecution() {
	// Step1: choose leader.
	request := w.request

	leader := worker.ChooseLeader(request.WorkId, request.AllParties)
	w.availableParties.add(w.myPid, 1)

	// Step2: If this node is the leader, sends check availability to everyone.
	if w.myPid.Id == leader.Id {
		w.doPreExecutionAsLeader()
	} else {
		w.doPreExecutionAsMember(leader)
	}
}

// Do preExecution as a leader for the tss work.
func (w *DefaultWorker) doPreExecutionAsLeader() {
	preWorkCache := w.preExecutionCache.GetAllMessages(w.workId)
	// Update availability from cache first.
	for _, tssMsg := range preWorkCache {
		if tssMsg.Type == common.TssMessage_AVAILABILITY_RESPONSE && tssMsg.AvailabilityResponseMessage.Answer == common.AvailabilityResponseMessage_YES {
			// update the availability.
			for _, p := range w.allParties {
				if p.Id == tssMsg.From {
					w.availableParties.add(p, 1)
					break
				}
			}
		}
	}

	for _, p := range w.allParties {
		if p.Id == w.myPid.Id {
			continue
		}

		// Only send request message to parties that has not sent a message to us.
		if w.availableParties.getParty(p.Id) == nil {
			tssMsg := common.NewAvailabilityRequestMessage(w.myPid.Id, p.Id, w.request.WorkId)
			log.Info("asking member")
			go w.dispatcher.UnicastMessage(p, tssMsg)
		}
	}

	// Waits for all members to respond.
	presignIds, selectedPids, err := w.waitForMemberResponse()
	if err != nil {
		var culprits []*tss.PartyID
		// Blame nodes that do not send messages
		for _, party := range w.allParties {
			if ok := w.availableParties.hasPartyId(party.Id); !ok {
				culprits = append(culprits, party)
			}
		}
		w.blameMgr.AddPreExecutionCulprit(append(culprits, w.myPid))

		log.Error("Leader: error while waiting for member response, err = ", err)
		w.leaderFinalized(false, nil, nil)
		w.callback.OnWorkFailed(w.request)
		return
	}

	w.leaderFinalized(true, presignIds, selectedPids)
}

func (w *DefaultWorker) waitForMemberResponse() ([]string, []*tss.PartyID, error) {
	if ok, presignIds, selectedPids := w.checkEnoughParticipants(); ok {
		// We have enough participants from cached message. No need to wait.
		return presignIds, selectedPids, nil
	}

	// Wait for everyone to reply or timeout.
	end := time.Now().Add(PreExecutionRequestWaitTime)
	for {
		now := time.Now()
		if now.After(end) {
			break
		}

		timeDiff := end.Sub(now)
		select {
		case <-time.After(timeDiff):
			// Time out, this returns failure except for 1 case:
			//
			// If this is a signing task and we have enough online (>= threshold + 1) but cannot find
			// the presigns set to do the signing, we have to do the presign first before doing signing
			if w.request.IsSigning() {
				// We have to do presign + signing ƒrom online nodes since we cannot find appropriate presign data.
				parties := w.availableParties.getPartyList(w.request.Threshold + 1)
				if len(parties) >= w.request.Threshold+1 {
					return nil, parties, nil
				}
			}
			return nil, nil, errors.New("timeout waiting for member response")

		case tssMsg := <-w.memberResponseCh:
			// Check if this member is one of the parties we know
			var party *tss.PartyID
			for _, p := range w.allParties {
				if p.Id == tssMsg.From {
					party = p
					break
				}
			}

			if party == nil {
				// Message can be from bad actor, continue to execute.
				log.Error("Cannot find party from", tssMsg.From)
				continue
			}

			if tssMsg.AvailabilityResponseMessage.MaxJob > 0 {
				w.availableParties.add(party, int(tssMsg.AvailabilityResponseMessage.MaxJob))
			}
		}

		if ok, presignIds, selectedPids := w.checkEnoughParticipants(); ok {
			return presignIds, selectedPids, nil
		}
	}

	return nil, nil, errors.New("cannot find enough members for this work")
}

// checkEnoughParticipants is a function called by the leader in the election to see if we have
// enough nodes to participate and find a common presign set.
func (w *DefaultWorker) checkEnoughParticipants() (bool, []string, []*tss.PartyID) {
	if w.availableParties.getLength() < w.request.GetMinPartyCount() {
		return false, nil, make([]*tss.PartyID, 0)
	}

	if w.request.IsSigning() {
		// Check if we can find a presign list that match this of nodes.
		presignIds, selectedPids := w.callback.GetAvailablePresigns(w.batchSize, w.request.N, w.availableParties.getAllPartiesMap())
		if len(presignIds) == w.batchSize {
			log.Info("checkEnoughParticipants: presignIds = ", presignIds, " batchSize = ", w.batchSize, " selectedPids = ", selectedPids)
			// Announce this as success and return
			return true, presignIds, selectedPids
		} else {
			// Otherwise, keep waiting
			return false, nil, make([]*tss.PartyID, 0)
		}
	} else {
		// Choose top parties with highest computing power.
		topParties, _ := w.availableParties.getTopParties(w.request.GetMinPartyCount())

		w.batchSize = w.request.BatchSize

		return true, nil, topParties
	}
}

// Finalize work as a leader and start execution.
func (w *DefaultWorker) leaderFinalized(success bool, presignIds []string, selectedPids []*tss.PartyID) {
	if !success { // Failure case
		msg := common.NewPreExecOutputMessage(w.myPid.Id, "", w.workId, false, presignIds, w.pIDs)
		go w.dispatcher.BroadcastMessage(w.pIDs, msg)
		return
	}

	// Get list of parties
	// pIDs := w.availableParties.getPartyList(w.request.GetMinPartyCount())
	w.pIDs = tss.SortPartyIDs(selectedPids)
	w.pIDsMap = pidsToMap(w.pIDs)

	// Broadcast success to everyone
	msg := common.NewPreExecOutputMessage(w.myPid.Id, "", w.workId, true, presignIds, w.pIDs)
	go w.dispatcher.BroadcastMessage(w.allParties, msg)

	workType := w.jobType
	if w.request.IsSigning() {
		if len(presignIds) > 0 {
			w.signingInput = w.callback.GetPresignOutputs(presignIds)
			if w.signingInput != nil && len(w.signingInput) > 0 {
				log.Info("Found a set of presign input")
			}
		} else {
			log.Info("Cannot find pre-generated presign. Doing presign work right now.")
			w.signingInput = nil
			workType = types.EcdsaPresign
		}
	}

	if err := w.executeWork(workType); err != nil {
		log.Error("Error when executing work", err)
	}
}

func (w *DefaultWorker) doPreExecutionAsMember(leader *tss.PartyID) {
	cachedMsgs := w.preExecutionCache.GetAllMessages(w.workId)

	// Check in the cache to see if the leader has sent a message to this node regarding the participants.
	for _, msg := range cachedMsgs {
		if msg.Type == common.TssMessage_PRE_EXEC_OUTPUT {
			log.Verbose("We have received participant list of work", w.workId)
			w.memberFinalized(msg.PreExecOutputMessage)
			return
		}
	}

	// Send a message to the leader.
	tssMsg := common.NewAvailabilityResponseMessage(w.myPid.Id, leader.Id, w.workId, common.AvailabilityResponseMessage_YES, w.maxJob)
	go w.dispatcher.UnicastMessage(leader, tssMsg)

	// Waits for response from the leader.
	select {
	case <-time.After(LeaderWaitTime):
		// TODO: Report as failure here.
		log.Error("member: leader wait timed out.")
		// Blame leader
		w.blameMgr.AddPreExecutionCulprit([]*tss.PartyID{leader})
		w.callback.OnWorkFailed(w.request)

	case msg := <-w.preExecMsgCh:
		w.memberFinalized(msg)
	}
}

func (w *DefaultWorker) onPreExecutionRequest(tssMsg *commonTypes.TssMessage) error {
	sender := w.getPartyIdFromString(tssMsg.From)
	if sender != nil {
		// TODO: Check that the sender is indeed the leader.
		// We receive a message from a leader to check our availability. Reply "Yes".
		responseMsg := common.NewAvailabilityResponseMessage(w.myPid.Id, tssMsg.From, w.workId, common.AvailabilityResponseMessage_YES, w.maxJob)
		responseMsg.AvailabilityResponseMessage.MaxJob = int32(w.maxJob)

		go w.dispatcher.UnicastMessage(sender, responseMsg)
	} else {
		return fmt.Errorf("cannot find party with id %s", tssMsg.From)
	}

	return nil
}

func (w *DefaultWorker) onPreExecutionResponse(tssMsg *commonTypes.TssMessage) error {
	w.memberResponseCh <- tssMsg
	return nil
}

// memberFinalized is called when all the participants have been finalized by the leader.
// We either start execution or finish this work.
func (w *DefaultWorker) memberFinalized(msg *common.PreExecOutputMessage) {
	if msg.Success {
		// TODO: Check validity of the presign ids. Make sure it is never used.

		// Check if we are in the list of participants or not
		join := false
		pIDs := make([]*tss.PartyID, 0, len(msg.Pids))
		for _, participant := range msg.Pids {
			if w.myPid.Id == participant {
				join = true
			}
			pIDs = append(pIDs, helper.GetPidFromString(participant, w.allParties))
		}

		w.pIDs = tss.SortPartyIDs(pIDs)
		w.pIDsMap = pidsToMap(w.pIDs)

		if join {
			workType := w.jobType
			if w.request.IsSigning() {
				if msg.PresignIds != nil && len(msg.PresignIds) > 0 {
					w.signingInput = w.callback.GetPresignOutputs(msg.PresignIds)
				} else {
					w.signingInput = nil
					workType = types.EcdsaPresign
				}
			}

			// We are one of the participants, execute the work
			if err := w.executeWork(workType); err != nil {
				log.Error("Error when executing work", err)
			}
		} else {
			// We are not in the participant list. Terminate this work. Nothing else to do.
			w.callback.OnNodeNotSelected(w.request)
		}
	} else { // msg.Success == false
		// This work fails because leader cannot find enough participants.
		w.callback.OnWorkFailed(w.request)
	}
}

func (w *DefaultWorker) Stop() {
	w.jobsLock.RLock()
	jobs := w.jobs
	w.jobsLock.RUnlock()

	for _, job := range jobs {
		if job != nil {
			job.Stop()
		}
	}
}

func pidsToMap(pids []*tss.PartyID) map[string]*tss.PartyID {
	res := make(map[string]*tss.PartyID)
	for _, pid := range pids {
		res[pid.Id] = pid
	}

	return res
}
