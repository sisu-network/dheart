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
	"github.com/puzpuzpuz/xsync"
	"github.com/sisu-network/lib/log"
)

type Connection struct {
	peerId  peer.ID
	addr    maddr.Multiaddr
	host    *host.Host
	streams *xsync.MapOf[protocol.ID, network.Stream]
	lock    sync.RWMutex
	status  int
}

func NewConnection(pID peer.ID, addr maddr.Multiaddr, host *host.Host) *Connection {
	return &Connection{
		peerId:  pID,
		addr:    addr,
		host:    host,
		streams: new(xsync.MapOf[protocol.ID, network.Stream]),
	}
}

func (con *Connection) ReleaseStream() {
	con.streams.Range(func(key protocol.ID, stream network.Stream) bool {
		stream.Reset()

		return true
	})
}

func (con *Connection) createStream(protocolId protocol.ID) (network.Stream, error) {
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
	stream, ok := con.streams.Load(protocolId)
	if !ok {
		var err error
		stream, err = con.createStream(protocolId)
		if err != nil {
			log.Warnf("Cannot create a new stream, err = %v", err)
			return err
		}
	}

	return WriteStreamWithBuffer(msg, stream)
}
