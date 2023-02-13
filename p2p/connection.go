package p2p

import (
	"context"
	"fmt"
	"sync"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	maddr "github.com/multiformats/go-multiaddr"
	"github.com/sisu-network/lib/log"
	"go.uber.org/atomic"
)

type Connection struct {
	peerId peer.ID
	addr   maddr.Multiaddr
	host   *host.Host

	streams *sync.Map

	creatingStream *atomic.Bool
}

func NewConnection(pID peer.ID, addr maddr.Multiaddr, host *host.Host) *Connection {
	return &Connection{
		peerId:         pID,
		addr:           addr,
		host:           host,
		streams:        &sync.Map{},
		creatingStream: atomic.NewBool(false),
	}
}

func (con *Connection) CreateStream(protocolId protocol.ID) (network.Stream, error) {
	if con.creatingStream.Load() {
		return nil, fmt.Errorf("Another stream is being created")
	}

	con.creatingStream.Store(true)
	defer con.creatingStream.Store(false)

	// Remove old stream if any
	old, ok := con.streams.Load(protocolId)
	if ok {
		log.Verbosef("Removing old stream for peer %s", con.peerId)
		s := old.(network.Stream)
		s.Close()
		con.streams.Delete(protocolId)
	}

	ctx, cancel := context.WithTimeout(context.Background(), TimeoutConnecting)
	defer cancel()
	stream, err := (*con.host).NewStream(ctx, con.peerId, protocolId)

	if err != nil {
		return nil, fmt.Errorf("fail to create new stream to peer: %s, %w", con.peerId, err)
	}

	con.streams.Store(protocolId, stream)
	return stream, nil
}

func (con *Connection) writeToStream(msg []byte, protocolId protocol.ID) error {
	value, ok := con.streams.Load(protocolId)
	if !ok {
		return fmt.Errorf("Connection is not ready. A stream is being created!")
	}

	return WriteStreamWithBuffer(msg, value.(network.Stream))
}
