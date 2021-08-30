package ecdsa

import (
	"fmt"
	"time"

	"github.com/sisu-network/dheart/types/common"
	commonTypes "github.com/sisu-network/dheart/types/common"
	"github.com/sisu-network/dheart/utils"
	"github.com/sisu-network/dheart/worker"
	"github.com/sisu-network/dheart/worker/helper"
	"github.com/sisu-network/tss-lib/tss"
)

// preExecution finds list of nodes who are available for signing this work.
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
	w.setAvailableParty(w.myPid)

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
					w.setAvailableParty(p)
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
		if w.getAvailableParty(p.Id) == nil {
			tssMsg := common.NewAvailabilityRequestMessage(w.myPid.Id, p.Id, w.request.WorkId)
			go w.dispatcher.UnicastMessage(p, tssMsg)
		}
	}

	if w.getAvailablePartyLength() < w.request.N {
		// Waits for all members to respond.
		w.waitForMemberResponse()
	}

	if w.getAvailablePartyLength() >= w.request.N {
		w.leaderFinalized()
	} else {
		// TODO: make callback that this preExecution fails.
	}
}

func (w *DefaultWorker) waitForMemberResponse() {
	// Wait for everyone to reply or timeout.
	end := time.Now().Add(PRE_EXECUTION_REQUEST_WAIT_TIME)
	for {
		now := time.Now()
		if now.After(end) {
			break
		}

		timeDiff := end.Sub(now)
		select {
		case <-time.After(timeDiff):
			// Timeout
			utils.LogVerbose("leader wait timeout")
			break

		case tssMsg := <-w.memberResponseCh:
			// Check if this member is one of the parties we know
			var party *tss.PartyID
			for _, p := range w.allParties {
				if p.Id == tssMsg.From {
					party = p
					break
				}
			}

			if party != nil {
				w.setAvailableParty(party)

				// TODO: Do check with database in case of signing a message. We only want to do pick up
				// participants who are in a presign set.
			} else {
				utils.LogError("Cannot find party from", tssMsg.From)
			}
		}

		if w.getAvailablePartyLength() >= w.request.N {
			return
		}
	}
}

// Finalize work as a leader and start execution.
func (w *DefaultWorker) leaderFinalized() {
	// Get list of parties
	pIDs := make([]*tss.PartyID, 0)

	w.preExecutionLock.Lock()
	for _, p := range w.availableParties {
		pIDs = append(pIDs, p)
	}
	w.pIDs = tss.SortPartyIDs(pIDs)
	w.preExecutionLock.Unlock()

	// Broadcast success to everyone
	msg := common.NewWorkParticipantsMessage(w.myPid.Id, "", w.workId, true, w.pIDs)
	go w.dispatcher.BroadcastMessage(w.pIDs, msg)

	w.executeWork()
}

func (w *DefaultWorker) setAvailableParty(p *tss.PartyID) {
	w.preExecutionLock.Lock()
	defer w.preExecutionLock.Unlock()

	w.availableParties[p.Id] = p
}

func (w *DefaultWorker) getAvailableParty(pid string) *tss.PartyID {
	w.preExecutionLock.RLock()
	defer w.preExecutionLock.RUnlock()

	return w.availableParties[pid]
}

func (w *DefaultWorker) getAvailablePartyLength() int {
	w.preExecutionLock.RLock()
	defer w.preExecutionLock.RUnlock()

	return len(w.availableParties)
}

func (w *DefaultWorker) doPreExecutionAsMember(leader *tss.PartyID) {
	cachedMsgs := w.preExecutionCache.GetAllMessages(w.workId)

	// Check in the cache to see if the leader has sent a message to this node regarding the participants.
	for _, msg := range cachedMsgs {
		if msg.Type == common.TssMessage_WORK_PARTICIPANTS {
			utils.LogVerbose("We have received participant list of work", w.workId)
			w.memberFinalized(msg.WorkParticipantsMessage)
			return
		}
	}

	// Send a message to the leader.
	tssMsg := common.NewAvailabilityResponseMessage(w.myPid.Id, leader.Id, w.workId, common.AvailabilityResponseMessage_YES)
	go w.dispatcher.UnicastMessage(leader, tssMsg)

	// Waits for response from the leader.
	select {
	case <-time.After(LEADER_WAIT_TIME):
		// TODO: Report as failure here.
		utils.LogError("member: leader wait timed out.")

	case msg := <-w.workParticipantCh:
		w.memberFinalized(msg)
	}
}

func (w *DefaultWorker) onPreExecutionRequest(tssMsg *commonTypes.TssMessage) error {
	sender := w.getPartyIdFromString(tssMsg.From)
	if sender != nil {
		// We receive a message from a leader to check our availability. Reply "Yes".
		responseMsg := common.NewAvailabilityResponseMessage(w.myPid.Id, tssMsg.From, w.workId, common.AvailabilityResponseMessage_YES)
		go w.dispatcher.UnicastMessage(sender, responseMsg)
	} else {
		utils.LogError("Cannot find party with id", tssMsg.From)
		return fmt.Errorf("Cannot find party with id %s", tssMsg.From)
	}

	return nil
}

func (w *DefaultWorker) onPreExecutionResponse(tssMsg *commonTypes.TssMessage) error {
	w.memberResponseCh <- tssMsg
	return nil
}

// memberFinalized is called when all the partcipants have been finalized by the leader.
// We either start execution or finish this work.
func (w *DefaultWorker) memberFinalized(msg *common.WorkParticipantsMessage) {
	if msg.Success {
		// Check if we are in the list of participants or not
		join := false
		pIDs := make([]*tss.PartyID, 0)
		for _, participant := range msg.Pids {
			if w.myPid.Id == participant {
				join = true
			}
			pIDs = append(pIDs, helper.GetPidFromString(participant, w.allParties))
		}

		w.pIDs = tss.SortPartyIDs(pIDs)

		if join {
			// We are one of the participants, execute the work
			w.executeWork()
		} else {
			// We are not in the participant list. Terminate this work. No thing else to do.
			w.callback.OnPreExecutionFinished(w.workId)
		}
	} else {
		// This work fails because leader cannot find enough participants.
		w.callback.OnWorkFailed(w.workId)
	}
}
