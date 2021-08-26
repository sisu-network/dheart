package core

import (
	"math/big"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/sisu-network/dheart/utils"
	"github.com/sisu-network/tss-lib/tss"
	tcrypto "github.com/tendermint/tendermint/crypto"
)

type Node struct {
	PeerId  peer.ID
	PubKey  tcrypto.PubKey
	PartyId *tss.PartyID
}

func NewNode(pubKey tcrypto.PubKey) *Node {
	p2pPubKey, err := crypto.UnmarshalSecp256k1PublicKey(pubKey.Bytes())
	if err != nil {
		utils.LogError(err)
		return nil
	}

	peerId, err := peer.IDFromPublicKey(p2pPubKey)

	return &Node{
		peerId, pubKey, tss.NewPartyID(peerId.String(), "", new(big.Int).SetBytes(pubKey.Bytes())),
	}
}
