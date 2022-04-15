package components

import (
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/sisu-network/dheart/core/message"
	"github.com/sisu-network/tss-lib/tss"

	wTypes "github.com/sisu-network/dheart/worker/types"
	"go.uber.org/atomic"
)

// This interface monitors all messages sent to a worker for execution. It saves what messages that
// this node has received and triggers a callback when some message is missing.
type MessageMonitor interface {
	Start()
	Stop()
	NewMessageReceived(msg tss.ParsedMessage, from *tss.PartyID)
}

type MessageMonitorCallback interface {
	OnMissingMesssageDetected(map[string][]string)
}

type DefaultMessageMonitor struct {
	jobType          wTypes.WorkType
	stopped          atomic.Bool
	timeout          time.Duration
	callback         MessageMonitorCallback
	pIDsMap          map[string]*tss.PartyID
	receivedMessages map[string][]bool

	lastReceivedTime time.Time
	lock             *sync.RWMutex
	allMessages      []string
}

func NewMessageMonitor(jobType wTypes.WorkType, callback MessageMonitorCallback, pIDsMap map[string]*tss.PartyID) MessageMonitor {
	allMessages := message.GetMessagesByWorkType(jobType)
	receivedMessages := make(map[string][]bool)
	for pid, _ := range pIDsMap {
		receivedMessages[pid] = make([]bool, len(allMessages))
	}

	return &DefaultMessageMonitor{
		jobType:          jobType,
		stopped:          *atomic.NewBool(false),
		lock:             &sync.RWMutex{},
		timeout:          time.Second * 1,
		callback:         callback,
		pIDsMap:          pIDsMap,
		receivedMessages: receivedMessages,
		allMessages:      message.GetMessagesByWorkType(jobType),
	}
}

func (m *DefaultMessageMonitor) Start() {
	for {
		select {
		case <-time.After(m.timeout):
			if m.stopped.Load() {
				return
			}

			m.lock.RLock()
			lastReceivedTime := m.lastReceivedTime
			m.lock.RUnlock()

			if lastReceivedTime.IsZero() || lastReceivedTime.Add(m.timeout).Before(time.Now()) {
				continue
			}

			m.findMissingMessages()
		}
	}
}

func (m *DefaultMessageMonitor) loop() {
}

func (m *DefaultMessageMonitor) Stop() {
	m.stopped.Store(true)
}

func (m *DefaultMessageMonitor) NewMessageReceived(msg tss.ParsedMessage, from *tss.PartyID) {
	// Check if our cache contains the pid
	m.lock.RLock()
	receivedArr := m.receivedMessages[msg.GetFrom().Id]
	m.lock.RUnlock()
	if receivedArr == nil {
		log.Warn("NewMessageReceived: cannot find pid in the receivedMessages, pid = ", msg.GetFrom().Id)
		return
	}

	m.lock.Lock()

	m.lastReceivedTime = time.Now()
	for i, s := range m.allMessages {
		if s == msg.Type() {
			receivedArr[i] = true
			break
		}
	}

	m.lock.Unlock()
}

func (m *DefaultMessageMonitor) findMissingMessages() {
	m.lock.RLock()
	// Make a copy of received messages
	receivedMessages := m.receivedMessages
	for key, value := range m.receivedMessages {
		receivedMessages[key] = value
	}
	m.lock.RUnlock()

	// Find the maximum message index that we have received.
	maxIndex := -1
	for pid := range m.pIDsMap {
		received := receivedMessages[pid]
		for i := range m.allMessages {
			if received[i] {
				if maxIndex < i {
					maxIndex = i
				}
			}
		}
	}

	// Enqueue all pids whose missing messages are smaller or equal maxIndex
	missingPids := make(map[string][]string)
	for pid := range m.pIDsMap {
		received := receivedMessages[pid]
		missingMsgs := make([]string, 0)
		for i := range m.allMessages {
			if i <= maxIndex && !received[i] {
				missingMsgs = append(missingMsgs, m.allMessages[i])
			}
		}
		if len(missingMsgs) > 0 {
			missingPids[pid] = missingMsgs
		}
	}

	if len(missingPids) > 0 {
		// Make a call back
		go m.callback.OnMissingMesssageDetected(missingPids)
	}
}
