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
)

type Connection struct {
	peerId peer.ID
	addr   maddr.Multiaddr
	host   *host.Host

	streams map[protocol.ID]network.Stream
	lock    sync.RWMutex
	status  int
}

func NewConnection(pID peer.ID, addr maddr.Multiaddr, host *host.Host) *Connection {
	return &Connection{
		peerId:  pID,
		addr:    addr,
		host:    host,
		streams: make(map[protocol.ID]network.Stream),
	}
}

func (con *Connection) ReleaseStream() {
	for _, stream := range con.streams {
		if stream != nil {
			stream.Reset()
		}
	}
}

func (con *Connection) createStream(protocolId protocol.ID) (network.Stream, error) {
	ctx, cancel := context.WithTimeout(context.Background(), TimeoutConnecting)
	defer cancel()
	stream, err := (*con.host).NewStream(ctx, con.peerId, protocolId)

	if err != nil {
		return nil, fmt.Errorf("fail to create new stream to peer: %s, %w", con.peerId, err)
	}

	con.streams[protocolId] = stream

	return stream, nil
}

func (con *Connection) writeToStream(msg []byte, protocolId protocol.ID) error {
	stream := con.streams[protocolId]
	if stream == nil {
		var err error
		stream, err = con.createStream(protocolId)
		if err != nil {
			log.Warnf("Cannot create a new stream, err = %v", err)
			return err
		}
	}

	con.lock.Lock()
	defer con.lock.Unlock()

	return WriteStreamWithBuffer(msg, stream)
}
