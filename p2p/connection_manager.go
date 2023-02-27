package p2p

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	libp2pPeer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/p2p/discovery/routing"
	maddr "github.com/multiformats/go-multiaddr"
	"github.com/puzpuzpuz/xsync"
	"github.com/sisu-network/lib/log"
	"go.uber.org/atomic"

	p2ptypes "github.com/sisu-network/dheart/p2p/types"
	types "github.com/sisu-network/dheart/p2p/types"
)

const (
	TSSProtocolID     protocol.ID = "/p2p-tss"
	PingProtocolID    protocol.ID = "/p2p-ping"
	KEY_secp256k1                 = "secp256k1"
	TimeoutConnecting             = time.Second * 20
)

type P2PDataListener interface {
	OnNetworkMessage(message *types.P2PMessage)
}

type ConnectionManager interface {
	// Starts this connection manager using private key as identity of the node.
	Start(privKeyBytes []byte, keyType string) error

	// Sends an array of byte to a particular peer.
	WriteToStream(pID libp2pPeer.ID, protocolId protocol.ID, msg []byte) error

	// Adds data listener for a protocol.
	AddListener(protocol protocol.ID, listener P2PDataListener)

	// Add a new set of peers
	AddPeers([]p2ptypes.Peer)

	IsReady() bool
}

// DefaultConnectionManager implements ConnectionManager interface.
type DefaultConnectionManager struct {
	config           types.ConnectionsConfig
	myNetworkId      libp2pPeer.ID
	host             host.Host
	port             int
	rendezvous       string
	connections      *xsync.MapOf[libp2pPeer.ID, *Connection]
	listenerLock     sync.RWMutex
	protocolListener map[protocol.ID]P2PDataListener
	ready            *atomic.Bool
	initPeers        []p2ptypes.Peer
	curPeers         []p2ptypes.Peer
}

func NewConnectionManager(config types.ConnectionsConfig, initPeers []p2ptypes.Peer) ConnectionManager {
	return &DefaultConnectionManager{
		config:           config,
		rendezvous:       config.Rendezvous,
		connections:      new(xsync.MapOf[libp2pPeer.ID, *Connection]),
		protocolListener: make(map[protocol.ID]P2PDataListener),
		ready:            atomic.NewBool(false),
		initPeers:        initPeers,
		curPeers:         initPeers,
	}
}

func (cm *DefaultConnectionManager) Start(privKeyBytes []byte, keyType string) error {
	log.Info("Starting connection manager. Config =", cm.connections)
	log.Info("keyType = ", keyType)

	ctx := context.Background()
	var p2pPriKey crypto.PrivKey
	var err error
	switch keyType {
	case "secp256k1":
		p2pPriKey, err = crypto.UnmarshalSecp256k1PrivateKey(privKeyBytes)
	case "ed25519":
		p2pPriKey, err = crypto.UnmarshalEd25519PrivateKey(privKeyBytes)
	}

	if err != nil {
		return err
	}

	selfUrl := fmt.Sprintf("/ip4/%s/tcp/%d", cm.config.Host, cm.config.Port)
	log.Info("selfUrl = ", selfUrl)

	listenAddr, err := maddr.NewMultiaddr(selfUrl)
	if err != nil {
		return err
	}

	host, err := libp2p.New(
		libp2p.ListenAddrs([]maddr.Multiaddr{listenAddr}...),
		libp2p.Identity(p2pPriKey),
	)
	if err != nil {
		log.Critical("Failed to get Peer ID from private key. err = ", err)
		return err
	}
	cm.host = host

	log.Info("My address = ", host.Addrs(), host.ID())

	// Set stream handlers
	host.SetStreamHandler(TSSProtocolID, cm.handleStream)
	host.SetStreamHandler(PingProtocolID, cm.handleStream)

	// Advertise this node and discover other nodes
	if err := cm.discover(ctx, host); err != nil {
		log.Critical("Failed to advertise and discover other nodes. err = ", err)
		return err
	}

	// Connect to predefined peers.
	log.Infof("cm.initPeers = %s", cm.initPeers)
	cm.createConnections(cm.initPeers)

	return nil
}

func (cm *DefaultConnectionManager) IsReady() bool {
	return cm.ready.Load()
}

func (cm *DefaultConnectionManager) AddListener(protocol protocol.ID, listener P2PDataListener) {
	cm.listenerLock.Lock()
	defer cm.listenerLock.Unlock()

	cm.protocolListener[protocol] = listener
}

func (cm *DefaultConnectionManager) RemoveListener(protocol protocol.ID, listener P2PDataListener) {
	cm.listenerLock.Lock()
	defer cm.listenerLock.Unlock()

	delete(cm.protocolListener, protocol)
	if cm.host != nil {
		cm.host.RemoveStreamHandler(protocol)
	}
}

func (cm *DefaultConnectionManager) getListener(protocol protocol.ID) P2PDataListener {
	cm.listenerLock.Lock()
	defer cm.listenerLock.Unlock()

	return cm.protocolListener[protocol]
}

func (cm *DefaultConnectionManager) handleStream(stream network.Stream) {
	peerIDString := stream.Conn().RemotePeer().String()
	protocol := stream.Protocol()

	for {
		// TODO: Add cancel channel here.
		dataBuf, err := ReadStreamWithBuffer(stream)

		if err != nil {
			log.Warn("Failed to read stream, err = ", err)
			// TODO: handle retry here.
			return
		}
		if dataBuf != nil {
			listener := cm.getListener(protocol)
			if listener == nil {
				// No listener. Ignore the message
				return
			}

			go func(peerIDString string, dataBuf []byte) {
				listener.OnNetworkMessage(&types.P2PMessage{
					FromPeerId: peerIDString,
					Data:       dataBuf,
				})
			}(peerIDString, dataBuf)
		}
	}
}

func (cm *DefaultConnectionManager) discover(ctx context.Context, host host.Host) error {
	kademliaDHT, err := dht.New(ctx, host)
	if err != nil {
		return fmt.Errorf("Failed to create DHT: %w", err)
	}

	if err = kademliaDHT.Bootstrap(ctx); err != nil {
		return fmt.Errorf("Failed to bootstrap DHT: %w", err)
	}

	routingDiscovery := routing.NewRoutingDiscovery(kademliaDHT)
	routingDiscovery.Advertise(ctx, cm.rendezvous)
	log.Info("Successfully announced!")

	return nil
}

func (cm *DefaultConnectionManager) createConnections(peers []p2ptypes.Peer) {
	// Creates connection objects.
	wg := &sync.WaitGroup{}
	for _, peer := range peers {
		// 1. Create a new connection instance
		peerMultAddr, err := maddr.NewMultiaddr(peer.Address)
		if err != nil {
			log.Errorf("Failed to add peer with address %s, err = %s", peer.Address, err)
			continue
		}

		addrInfo, err := libp2pPeer.AddrInfoFromP2pAddr(peerMultAddr)
		if err != nil {
			log.Errorf("Failed to get addrInfo %s, err = %s", peer.Address, err)
			continue
		}

		conn := NewConnection(addrInfo.ID, peerMultAddr, &cm.host)
		cm.connections.Store(addrInfo.ID, conn)

		// 2. Attempts to connect to every bootstrapped peers.
		log.Info("Trying to create connections with peers...")
		wg.Add(1)
		go func(addrInfo *libp2pPeer.AddrInfo) {
			defer wg.Done()

			// Retry 50 times at max.
			for i := 0; i < 50; i++ {
				if err := cm.host.Connect(context.Background(), *addrInfo); err != nil {
					log.Warnf("Error while connecting to node %q: %-v", peerMultAddr, err)
					time.Sleep(time.Second * 3)
				} else {
					log.Infof("%s Connection established with bootstrap node: %q", cm.myNetworkId, peerMultAddr)
					break
				}
			}
		}(addrInfo)
	}
	wg.Wait()

	cm.ready.Store(true)
}

func (cm *DefaultConnectionManager) WriteToStream(pID libp2pPeer.ID, protocolId protocol.ID, msg []byte) error {
	conn, ok := cm.connections.Load(pID)
	if !ok {
		log.Error("Connection to pid not found, pid = ", pID)
		return errors.New("pID not found")
	}

	err := conn.writeToStream(msg, protocolId)
	if err != nil {
		log.HighVerbosef("%s failed writing to stream, err = ", cm.myNetworkId, err)
	}

	return err
}

func (cm *DefaultConnectionManager) AddPeers(peers []p2ptypes.Peer) {
	cm.createConnections(peers)
}
