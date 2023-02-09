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

const (
	CONNECTION_STATUS_NOT_CONNECTED = iota
	CONNECTION_STATUS_CONNECTED
)

type Connection struct {
	peerId peer.ID
	addr   maddr.Multiaddr
	host   *host.Host

	streams    map[protocol.ID]network.Stream
	statusLock sync.RWMutex
	status     int
}

func NewConnection(pID peer.ID, addr maddr.Multiaddr, host *host.Host) *Connection {
	return &Connection{
		peerId:  pID,
		addr:    addr,
		host:    host,
		streams: make(map[protocol.ID]network.Stream),
		status:  CONNECTION_STATUS_NOT_CONNECTED,
	}
}

func (con *Connection) ReleaseStream() {
	for _, stream := range con.streams {
		if stream != nil {
			stream.Reset()
		}
	}
}

func (con *Connection) setStatus(status int) {
	con.statusLock.Lock()
	defer con.statusLock.Unlock()

	con.status = status
}

func (con *Connection) getStatus(status int) int {
	con.statusLock.Lock()
	defer con.statusLock.Unlock()

	return con.status
}

func (con *Connection) createStream(protocolId protocol.ID) (network.Stream, error) {
	ctx, cancel := context.WithTimeout(context.Background(), TimeoutConnecting)
	defer cancel()
	stream, err := (*con.host).NewStream(ctx, con.peerId, protocolId)

	if err != nil {
		return nil, fmt.Errorf("fail to create new stream to peer: %s, %w", con.peerId, err)
	}
	return stream, nil
}

func (con *Connection) writeToStream(msg []byte, protocolId protocol.ID) error {
	stream := con.streams[protocolId]
	if stream == nil {
		newStream, err := con.createStream(protocolId)
		if err != nil {
			log.Warnf("Cannot create a new stream, err = %v", err)
			return err
		}

		stream = newStream
		con.streams[protocolId] = newStream
	}

	return WriteStreamWithBuffer(msg, stream)
}
