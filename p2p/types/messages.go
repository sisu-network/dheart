package types

import (
	"github.com/libp2p/go-libp2p-core/protocol"
)

type P2PMessage struct {
	FromPeerId string
	Data       []byte
}

type Peer struct {
	Address    string `toml:"address"`
	PubKey     string `toml:"pubkey"`
	PubKeyType string `toml:"pubkey_type"`
}

type ConnectionsConfig struct {
	Host           string `toml:"host"`
	Port           int    `toml:"port"`
	Rendezvous     string `toml:"rendezvous"`
	Protocol       protocol.ID
	Peers          []*Peer `toml:"peers"`
	PrivateKeyType string
}
