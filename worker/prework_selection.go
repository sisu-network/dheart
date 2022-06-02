package worker

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"time"

	enginecache "github.com/sisu-network/dheart/core/cache"
	corecomponents "github.com/sisu-network/dheart/core/components"
	"github.com/sisu-network/dheart/core/config"
	"github.com/sisu-network/dheart/db"
	"github.com/sisu-network/dheart/types/common"
	commonTypes "github.com/sisu-network/dheart/types/common"
	"github.com/sisu-network/dheart/utils"
	"github.com/sisu-network/dheart/worker/helper"
	"github.com/sisu-network/dheart/worker/interfaces"
	"github.com/sisu-network/dheart/worker/types"
	"github.com/sisu-network/lib/log"
	"github.com/sisu-network/tss-lib/tss"
	"go.uber.org/atomic"
)

type SelectionFailureReason int64

const (
	SelectionTimeout SelectionFailureReason = iota
)

type SelectionResult struct {
	Success        bool
	IsNodeExcluded bool
	FailureResason SelectionFailureReason

	PresignIds   []string
	SelectedPids []*tss.PartyID
}

type PreworkSelection struct {
	///////////////////////
	// Immutable data.
	///////////////////////
	request          *types.WorkRequest
	allParties       []*tss.PartyID
	myPid            *tss.PartyID
	db               db.Database
	dispatcher       interfaces.MessageDispatcher
	callback         func(SelectionResult)
	preExecMsgCh     chan *common.PreExecOutputMessage
	memberResponseCh chan *common.TssMessage
	cfg              config.TimeoutConfig

	///////////////////////
	// Mutable data.
	///////////////////////
	leader          *tss.PartyID
	presignsManager corecomponents.AvailablePresigns
	// List of parties who indicate that they are available for current tss work.
	availableParties *AvailableParties

	// Cache all tss update messages when some parties start executing while this node has not.
	stopped *atomic.Bool
}

func NewPreworkSelection(request *types.WorkRequest, allParties []*tss.PartyID, myPid *tss.PartyID,
	db db.Database, preExecutionCache *enginecache.MessageCache, dispatcher interfaces.MessageDispatcher,
	presignsManager corecomponents.AvailablePresigns, cfg config.TimeoutConfig, callback func(SelectionResult)) *PreworkSelection {

	leader := chooseLeader(request.WorkId, request.AllParties)
	return &PreworkSelection{
		request:          request,
		allParties:       allParties,
		db:               db,
		availableParties: NewAvailableParties(),
		myPid:            myPid,
		leader:           leader,
		dispatcher:       dispatcher,
		preExecMsgCh:     make(chan *commonTypes.PreExecOutputMessage, 1),
		presignsManager:  presignsManager,
		memberResponseCh: make(chan *commonTypes.TssMessage, len(allParties)),
		callback:         callback,
		stopped:          atomic.NewBool(false),
		cfg:              cfg,
	}
}

func chooseLeader(workId string, parties []*tss.PartyID) *tss.PartyID {
	keyStore := make(map[string]int)
	sortedHashes := make([]string, len(parties))

	for i, party := range parties {
		sum := sha256.Sum256([]byte(party.Id + workId))
		encodedSum := hex.EncodeToString(sum[:])

		keyStore[encodedSum] = i
		sortedHashes[i] = encodedSum
	}

	sort.Strings(sortedHashes)

	return parties[keyStore[sortedHashes[0]]]
}

func (s *PreworkSelection) Init() {
	s.availableParties.add(s.myPid, 1)
}

func (s *PreworkSelection) Run(cachedMsgs []*commonTypes.TssMessage) {
	if s.myPid.Id == s.leader.Id {
		s.doPreExecutionAsLeader(cachedMsgs)
	} else {
		s.doPreExecutionAsMember(s.leader, cachedMsgs)
	}
}

func (s *PreworkSelection) Stop() {
	s.stopped.Store(true)
}

////////////////////////////////////////////////////////////////////////////
// LEADER
////////////////////////////////////////////////////////////////////////////

func (s *PreworkSelection) doPreExecutionAsLeader(cachedMsgs []*commonTypes.TssMessage) {
	// Update availability from cache first.
	for _, tssMsg := range cachedMsgs {
		if tssMsg.Type == common.TssMessage_AVAILABILITY_RESPONSE && tssMsg.AvailabilityResponseMessage.Answer == common.AvailabilityResponseMessage_YES {
			// update the availability.
			for _, p := range s.allParties {
				if p.Id == tssMsg.From {
					log.Verbose("Leader: Pid", p.Id, "has responded to us from a message in the cache for workId = ", s.request.WorkId)
					s.availableParties.add(p, 1)
					break
				}
			}
		}
	}

	// Check if we have received enough response but stored in the cache.
	if success, presignIds, selectedPids := s.checkEnoughParticipants(); success {
		s.leaderFinalized(true, presignIds, selectedPids)
		return
	}

	for _, p := range s.allParties {
		if p.Id == s.myPid.Id {
			continue
		}

		// Only send request message to parties that has not sent a message to us.
		if s.availableParties.getParty(p.Id) == nil {
			tssMsg := common.NewAvailabilityRequestMessage(s.myPid.Id, p.Id, s.request.WorkId)
			log.Verbose("Leader: sending query to ", p.Id, "for workId = ", s.request.WorkId)
			go s.dispatcher.UnicastMessage(p, tssMsg)
		}
	}

	// Waits for all members to respond.
	presignIds, selectedPids, err := s.waitForMemberResponse()
	if err != nil {
		log.Error("Leader: error while waiting for member response, err = ", err)
		s.leaderFinalized(false, nil, nil)
		return
	}

	s.leaderFinalized(true, presignIds, selectedPids)
}

func (s *PreworkSelection) waitForMemberResponse() ([]string, []*tss.PartyID, error) {
	// Wait for everyone to reply or timeout.
	end := time.Now().Add(s.cfg.SelectionLeaderTimeout)
	for {
		now := time.Now()
		if now.After(end) {
			break
		}

		timeDiff := end.Sub(now)
		select {
		case <-time.After(timeDiff):
			if s.request.IsSigning() && s.availableParties.Length() >= s.request.Threshold+1 {
				log.Info("Wait timeouted for signing but we got enough participants for presign.")
				// We have enough online participants but cannot find a presign set for all of them. This
				// still returns success and we do a presign round.
				selectedPids := s.availableParties.getPartyList(s.request.Threshold, s.myPid)
				selectedPids = append(selectedPids, s.myPid)
				return nil, selectedPids, nil
			}

			log.Info("LEADER: timeout")

			return nil, nil, errors.New("timeout: cannot find enough members for this work")

		case tssMsg := <-s.memberResponseCh:
			log.Info("LEADER: There is a member response, from: ", tssMsg.From)
			// Check if this member is one of the parties we know
			var party *tss.PartyID
			for _, p := range s.allParties {
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

			if tssMsg.AvailabilityResponseMessage.Answer == commonTypes.AvailabilityResponseMessage_YES {
				s.availableParties.add(party, 1)
				// TODO: Check if this is a new member to save one call for checkEnoughParticipants
				if ok, presignIds, selectedPids := s.checkEnoughParticipants(); ok {
					log.Info("Leader: selectedPids found")
					return presignIds, selectedPids, nil
				}
			}
		}
	}

	return nil, nil, errors.New("cannot find enough members for this work")
}

// checkEnoughParticipants is a function called by the leader in the election to see if we have
// enough nodes to participate and find a common presign set.result.PresignIds
func (s *PreworkSelection) checkEnoughParticipants() (bool, []string, []*tss.PartyID) {
	if s.availableParties.Length() < s.request.GetMinPartyCount() {
		return false, nil, make([]*tss.PartyID, 0)
	}

	if s.request.IsEddsa() {
		return true, nil, s.availableParties.getPartyList(s.request.Threshold, s.myPid)
	}

	if s.request.IsSigning() {
		batchSize := s.request.BatchSize
		// Check if we can find a presign list that match this of nodes.
		presignIds, selectedPids := s.presignsManager.GetAvailablePresigns(batchSize, s.request.N, s.availableParties.getAllPartiesMap())
		if len(presignIds) == batchSize {
			log.Info("We found a presign set: presignIds = ", presignIds, " batchSize = ", batchSize, " selectedPids = ", selectedPids)
			// Announce this as success and return
			return true, presignIds, selectedPids
		} else if s.availableParties.Length() == len(s.allParties) {
			selected := s.availableParties.getPartyList(s.request.Threshold, s.myPid)
			selected = append(selected, s.myPid) // include this leader in the list of final selection
			return true, nil, selected
		} else {
			// We cannot find enough presign, keep waiting.
			return false, nil, nil
		}
	} else {
		// Choose top parties with highest computing power.
		topParties, _ := s.availableParties.getTopParties(s.request.GetMinPartyCount())

		return true, nil, topParties
	}
}

// Finalize work as a leader and start execution.
func (s *PreworkSelection) leaderFinalized(success bool, presignIds []string, selectedPids []*tss.PartyID) {
	log.Verbose("Leader finalized, success = ", success)

	workId := s.request.WorkId
	if !success { // Failure case
		msg := common.NewPreExecOutputMessage(s.myPid.Id, "", workId, false, nil, nil)
		go s.dispatcher.BroadcastMessage(s.allParties, msg)

		s.broadcastResult(SelectionResult{
			Success: false,
		})
		return
	}

	// Broadcast success to everyone
	msg := common.NewPreExecOutputMessage(s.myPid.Id, "", workId, true, presignIds, selectedPids)
	log.Info("Leader: Broadcasting PreExecOutput to everyone...")
	go s.dispatcher.BroadcastMessage(s.allParties, msg)

	s.broadcastResult(SelectionResult{
		Success:      true,
		SelectedPids: selectedPids,
		PresignIds:   presignIds,
	})
}

////////////////////////////////////////////////////////////////////////////
// MEMBER
////////////////////////////////////////////////////////////////////////////

func (s *PreworkSelection) doPreExecutionAsMember(leader *tss.PartyID, cachedMsgs []*commonTypes.TssMessage) {
	// Check in the cache to see if the leader has sent a message to this node regarding the participants.
	for _, msg := range cachedMsgs {
		if msg.Type == common.TssMessage_PRE_EXEC_OUTPUT {
			log.Verbose("We have received final OUTCOME, work id = ", s.request.WorkId)
			s.memberFinalized(msg.PreExecOutputMessage)
			return
		}
	}

	// Send a message to the leader.
	tssMsg := common.NewAvailabilityResponseMessage(s.myPid.Id, leader.Id, s.request.WorkId, common.AvailabilityResponseMessage_YES, 1)
	log.Verbose("Member: Sending response message to the leader, workId = ", s.request.WorkId)
	go s.dispatcher.UnicastMessage(leader, tssMsg)

	// Waits for response from the leader.
	select {
	case <-time.After(s.cfg.SelectionMemberTimeout):
		// TODO: Report as failure here.
		log.Error("member: leader wait timed out.")
		// Blame leader

		s.broadcastResult(SelectionResult{
			Success:        false,
			FailureResason: SelectionTimeout,
		})

	case msg := <-s.preExecMsgCh:
		s.memberFinalized(msg)
	}
}

// memberFinalized is called when all the participants have been finalized by the leader.
// We either start execution or finish this work.
func (s *PreworkSelection) memberFinalized(msg *common.PreExecOutputMessage) {
	log.Verbose("memberFinalized, success = ", msg.Success)

	if msg.Success {
		// Do basic message validation
		if !s.validateLeaderSelection(msg) {
			s.broadcastResult(SelectionResult{
				Success: false,
			})
			return
		}

		// 1. Retrieve list of []*tss.PartyId from the pid strings.
		join := false
		pIDs := make([]*tss.PartyID, 0, len(msg.Pids))
		for _, participant := range msg.Pids {
			if s.myPid.Id == participant {
				join = true
			}
			partyId := helper.GetPidFromString(participant, s.allParties)
			pIDs = append(pIDs, partyId)
		}

		if join {
			if s.request.IsKeygen() || (s.request.IsSigning() && len(msg.PresignIds) == 0) {
				s.broadcastResult(SelectionResult{
					Success:      true,
					SelectedPids: pIDs,
				})
				return
			}

			if s.request.IsSigning() && len(msg.PresignIds) > 0 {
				// Check if the leader is giving us a valid presign id set.
				signingInput, err := s.db.LoadPresign(msg.PresignIds)
				if err != nil || len(signingInput) == 0 {
					log.Error("Cannot load presign, err =", err, " len(signingInput) = ", len(signingInput))
					s.broadcastResult(SelectionResult{
						Success: false,
					})
					return
				} else {
					s.broadcastResult(SelectionResult{
						Success:      true,
						SelectedPids: pIDs,
						PresignIds:   msg.PresignIds,
					})
				}
			}
		} else {
			// We are not in the participant list. Terminate this work. Nothing else to do.
			s.broadcastResult(SelectionResult{
				Success:        true,
				SelectedPids:   pIDs,
				IsNodeExcluded: true,
			})
		}
	} else { // msg.Success == false
		// This work fails because leader cannot find enough participants.
		s.broadcastResult(SelectionResult{
			Success: false,
		})
	}
}

func (s *PreworkSelection) validateLeaderSelection(msg *common.PreExecOutputMessage) bool {
	if s.request.IsKeygen() && len(msg.Pids) != len(s.allParties) {
		return false
	}

	if (s.request.IsKeygen() || s.request.IsSigning()) && len(msg.Pids) < s.request.Threshold+1 {
		// Not enough pids.
		return false
	}

	for _, pid := range msg.Pids {
		partyId := helper.GetPidFromString(pid, s.allParties)
		if partyId == nil {
			return false
		}
	}

	return true
}

func (s *PreworkSelection) broadcastResult(result SelectionResult) {
	s.stopped.Store(true)

	s.callback(result)
}

func (s *PreworkSelection) ProcessNewMessage(tssMsg *commonTypes.TssMessage) error {
	switch tssMsg.Type {
	case common.TssMessage_AVAILABILITY_REQUEST:
		if err := s.onPreExecutionRequest(tssMsg); err != nil {
			return fmt.Errorf("error when processing execution request %w", err)
		}
	case common.TssMessage_AVAILABILITY_RESPONSE:
		if err := s.onPreExecutionResponse(tssMsg); err != nil {
			return fmt.Errorf("error when processing execution response %w", err)
		}
	case common.TssMessage_PRE_EXEC_OUTPUT:
		// This output of workParticipantCh is called only once. We do checking for pids length to
		// make sure we only send message to this channel once.
		s.preExecMsgCh <- tssMsg.PreExecOutputMessage

	default:
		return errors.New("defaultPreworkSelection: invalid message " + tssMsg.Type.String())
	}

	return nil
}

func (s *PreworkSelection) onPreExecutionRequest(tssMsg *commonTypes.TssMessage) error {
	sender := utils.GetPartyIdFromString(tssMsg.From, s.allParties)

	if sender != nil {
		// TODO: Check that the sender is indeed the leader.
		// We receive a message from a leader to check our availability. Reply "Yes".
		log.Info("Member: Responding YES to leader's request")
		responseMsg := common.NewAvailabilityResponseMessage(s.myPid.Id, tssMsg.From, s.request.WorkId,
			common.AvailabilityResponseMessage_YES, 1)
		responseMsg.AvailabilityResponseMessage.MaxJob = int32(1)

		go s.dispatcher.UnicastMessage(sender, responseMsg)
	} else {
		return fmt.Errorf("cannot find party with id %s", tssMsg.From)
	}

	return nil
}

func (s *PreworkSelection) onPreExecutionResponse(tssMsg *commonTypes.TssMessage) error {
	// TODO: Check if we have finished selection process. Otherwise, this could create a blocking
	// operation.
	s.memberResponseCh <- tssMsg
	return nil
}
