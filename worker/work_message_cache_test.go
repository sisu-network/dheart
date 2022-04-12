package worker

import (
	"testing"

	"github.com/sisu-network/dheart/types/common"
	"github.com/stretchr/testify/require"
)

func TestWorkMessageCache(t *testing.T) {
	cache := NewWorkMessageCache(2)

	node1Msg1 := &common.SignedMessage{
		From:      "node1",
		Signature: []byte("node1-sig1"),
	}
	node1Msg2 := &common.SignedMessage{
		From:      "node1",
		Signature: []byte("node1-sig2"),
	}

	cache.Add("node1Msg1", node1Msg1)
	cache.Add("node1Msg2", node1Msg2)

	// Node 2
	node2Msg1 := &common.SignedMessage{
		From:      "node2",
		Signature: []byte("node2-sig1"),
	}
	cache.Add("node2Msg1", node2Msg1)

	require.Equal(t, []byte("node1-sig1"), cache.Get("node1", "node1Msg1").Signature)
	require.Equal(t, []byte("node1-sig2"), cache.Get("node1", "node1Msg2").Signature)
	require.Equal(t, []byte("node2-sig1"), cache.Get("node2", "node2Msg1").Signature)

	// Now add a new message to node1, it should repalce the node1Msg1 in the cache
	node1Msg3 := &common.SignedMessage{
		From:      "node1",
		Signature: []byte("node1-sig3"),
	}
	cache.Add("node1Msg3", node1Msg3)

	require.Nil(t, cache.Get("node1", "node1Msg1"))
	require.Equal(t, []byte("node1-sig3"), cache.Get("node1", "node1Msg3").Signature)
}
