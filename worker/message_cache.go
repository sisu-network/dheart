package worker

import (
	"sync"

	commonTypes "github.com/sisu-network/dheart/types/common"
)

const (
	MaxMessagePerNode = 64
)

type CacheValue struct {
	msgs []*commonTypes.TssMessage
}

// A cache that stores all messages sent to this node even before a worker starts or before a worker
// start execution and helps prevent message loss. The cache has a limit of number of messages PER
// VALIDATOR since we want to avoid bad actors spamming our node with fake tss work.
type MessageCache struct {
	cache     map[string]*CacheValue
	cacheLock *sync.RWMutex
}

func NewMessageCache() *MessageCache {
	return &MessageCache{
		cache:     make(map[string]*CacheValue),
		cacheLock: &sync.RWMutex{},
	}
}

func (c *MessageCache) AddMessage(msg *commonTypes.TssMessage) {
	c.cacheLock.Lock()
	defer c.cacheLock.Unlock()

	value := c.cache[msg.From]
	if value == nil {
		value = &CacheValue{}
	}

	if len(value.msgs) >= MaxMessagePerNode {
		// Remove the first element
		value.msgs = value.msgs[1:]
	}
	value.msgs = append(value.msgs, msg)

	c.cache[msg.From] = value
}

func (c *MessageCache) PopAllMessages(workId string) []*commonTypes.TssMessage {
	return c.getAllMessages(workId, true)
}

func (c *MessageCache) GetAllMessages(workId string) []*commonTypes.TssMessage {
	return c.getAllMessages(workId, false)
}

func (c *MessageCache) getAllMessages(workId string, update bool) []*commonTypes.TssMessage {
	c.cacheLock.Lock()
	defer c.cacheLock.Unlock()

	result := make([]*commonTypes.TssMessage, 0)
	newList := make([]*commonTypes.TssMessage, 0)

	for _, value := range c.cache {
		if value == nil {
			continue
		}

		for _, msg := range value.msgs {
			if msg.WorkId == workId {
				result = append(result, msg)
			} else {
				// Remove the selected messages from the list.
				newList = append(newList, msg)
			}
		}

		if update {
			value.msgs = newList
		}
	}

	return result
}
