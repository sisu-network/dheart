package core

import (
	"math/big"
	"sort"

	ctypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/sisu-network/dheart/utils"
	"github.com/sisu-network/tss-lib/tss"
)

type Node struct {
	PeerId  peer.ID
	PubKey  ctypes.PubKey
	PartyId *tss.PartyID
}

func NewNode(pubKey ctypes.PubKey) *Node {
	p2pPubKey, err := crypto.UnmarshalSecp256k1PublicKey(pubKey.Bytes())
	if err != nil {
		utils.LogError(err)
		return nil
	}

	peerId, err := peer.IDFromPublicKey(p2pPubKey)
	if err != nil {
		utils.LogError("Cannot convert pubkey to peerId")
		return nil
	}

	return &Node{
		peerId, pubKey, tss.NewPartyID(peerId.String(), "", new(big.Int).SetBytes(pubKey.Bytes())),
	}
}

func NewNodes(tPubKeys []ctypes.PubKey) []*Node {
	nodes := make([]*Node, len(tPubKeys))
	pids := make([]*tss.PartyID, len(tPubKeys))

	for i, pubKey := range tPubKeys {
		node := NewNode(pubKey)
		nodes[i] = node
		pids[i] = node.PartyId
	}

	// Sort nodes by partyId
	tss.SortPartyIDs(pids)
	sort.SliceStable(nodes, func(i, j int) bool {
		return nodes[i].PartyId.Index < nodes[j].PartyId.Index
	})

	return nodes
}
