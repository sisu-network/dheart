package ecdsa

import (
	"strings"
	"sync"

	"github.com/sisu-network/lib/log"
	"github.com/sisu-network/tss-lib/tss"
)

type Monitor interface {
	StoreMessage(msgKey string, msg tss.Message)
	GetMessage(msgKey string) (tss.Message, bool)
}

// DefaultWorkerMonitor caching tss messages which are produced by this party or received from other parties
type DefaultWorkerMonitor struct {
	MessageCache map[string]tss.Message
	cacheLock    sync.RWMutex
}

func NewDefaultWorkerMonitor() *DefaultWorkerMonitor {
	return &DefaultWorkerMonitor{
		MessageCache: make(map[string]tss.Message),
		cacheLock:    sync.RWMutex{},
	}
}

// StoreMessage stores produced/received tss messages, overwrite old message if duplicated. It's thread-safe function
func (m *DefaultWorkerMonitor) StoreMessage(msgKey string, msg tss.Message) {
	m.cacheLock.Lock()
	defer m.cacheLock.Unlock()

	log.Info("stored tss message with key ", msgKey)
	m.MessageCache[msgKey] = msg
}

// GetMessage gets tss-lib message by msg key
func (m *DefaultWorkerMonitor) GetMessage(msgKey string) (tss.Message, bool) {
	m.cacheLock.RLock()
	defer m.cacheLock.RUnlock()

	msg, ok := m.MessageCache[msgKey]
	return msg, ok
}

func GetCacheMsgKeyForTSSMsg(msg tss.Message) string {
	to := ""
	if !msg.IsBroadcast() {
		to = msg.GetTo()[0].GetId()
	}

	return GetCacheMsgKey(msg.Type(), msg.GetFrom().GetId(), to)
}

func GetCacheMsgKey(tssMsgType, from, to string) string {
	return strings.Join([]string{tssMsgType, from, to}, "-")
}
