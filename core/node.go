package core

import (
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/sisu-network/tss-lib/tss"
	tcrypto "github.com/tendermint/tendermint/crypto"
)

type Node struct {
	PeerId  peer.ID
	PubKey  tcrypto.PubKey
	PartyId *tss.PartyID
}
