package p2p

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	discovery "github.com/libp2p/go-libp2p-discovery"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	maddr "github.com/multiformats/go-multiaddr"
	"github.com/sisu-network/dheart/utils"
)

const (
	TSSProtocolID     protocol.ID = "/p2p-tss"
	PingProtocolID    protocol.ID = "/p2p-ping"
	KEY_secp256k1                 = "secp256k1"
	TimeoutConnecting             = time.Second * 20
)

type P2PMessage struct {
	FromPeerId string
	Data       []byte
}

type ConnectionsConfig struct {
	Host           string `toml:"host"`
	Port           int    `toml:"port"`
	Rendezvous     string `toml:"rendezvous"`
	Protocol       protocol.ID
	BootstrapPeers []string `toml:"peers"`
	PrivateKeyType string
}

type P2PDataListener interface {
	OnNetworkMessage(message *P2PMessage)
}

type ConnectionManager interface {
	// Starts this connection manager using private key as identity of the node.
	Start(privKeyBytes []byte, keyType string) error

	// Sends an array of byte to a particular peer.
	WriteToStream(pID peer.ID, protocolId protocol.ID, msg []byte) error

	// Adds data listener for a protocol.
	AddListener(protocol protocol.ID, listener P2PDataListener)
}

// DefaultConnectionManager implements ConnectionManager interface.
type DefaultConnectionManager struct {
	config           ConnectionsConfig
	myNetworkId      peer.ID
	host             host.Host
	port             int
	rendezvous       string
	bootstrapPeers   []maddr.Multiaddr
	connections      map[peer.ID]*Connection
	listenerLock     sync.RWMutex
	protocolListener map[protocol.ID]P2PDataListener
	statusManager    StatusManager
}

func NewConnectionManager(config ConnectionsConfig) ConnectionManager {
	return &DefaultConnectionManager{
		config:           config,
		rendezvous:       config.Rendezvous,
		connections:      make(map[peer.ID]*Connection),
		protocolListener: make(map[protocol.ID]P2PDataListener),
	}
}

func (cm *DefaultConnectionManager) Start(privKeyBytes []byte, keyType string) error {
	utils.LogInfo("Starting connection manager. Config =", cm.connections)
	utils.LogInfo("keyType = ", keyType)

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
	utils.LogInfo("selfUrl = ", selfUrl)

	listenAddr, err := maddr.NewMultiaddr(selfUrl)
	if err != nil {
		return err
	}

	// Initialize peers
	utils.LogInfo("cm.config.BootstrapPeers = ", cm.config.BootstrapPeers)
	cm.bootstrapPeers = make([]maddr.Multiaddr, len(cm.config.BootstrapPeers))
	for i, peerString := range cm.config.BootstrapPeers {
		peer, err := maddr.NewMultiaddr(peerString)
		if err != nil {
			return err
		}

		cm.bootstrapPeers[i] = peer
	}

	host, err := libp2p.New(ctx,
		libp2p.ListenAddrs([]maddr.Multiaddr{listenAddr}...),
		libp2p.Identity(p2pPriKey),
	)
	if err != nil {
		utils.LogCritical("Failed to get Peer ID from private key. err = ", err)
		return err
	}
	cm.host = host

	utils.LogInfo("My address = ", host.Addrs(), host.ID())

	// Set stream handlers
	host.SetStreamHandler(TSSProtocolID, cm.handleStream)
	host.SetStreamHandler(PingProtocolID, cm.handleStream)

	// Advertise this node and discover other nodes
	if err := cm.discover(ctx, host); err != nil {
		utils.LogCritical("Failed to advertise and discover other nodes. err = ", err)
		return err
	}

	// Connect to predefined peers.
	cm.createConnections(ctx)

	// Create status manager
	cm.initializeStatusManager()

	return nil
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
	for {
		peerIDString := stream.Conn().RemotePeer().String()
		protocol := stream.Protocol()

		// TODO: Add cancel channel here.
		dataBuf, err := ReadStreamWithBuffer(stream)

		if err != nil {
			utils.LogWarn(err)
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
				listener.OnNetworkMessage(&P2PMessage{
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

	routingDiscovery := discovery.NewRoutingDiscovery(kademliaDHT)
	discovery.Advertise(ctx, routingDiscovery, cm.rendezvous)
	utils.LogInfo("Successfully announced!")

	return nil
}

func (cm *DefaultConnectionManager) createConnections(ctx context.Context) {
	// Creates connection objects.
	for _, peerAddr := range cm.bootstrapPeers {
		addrInfo, _ := peer.AddrInfoFromP2pAddr(peerAddr)
		conn := NewConnection(addrInfo.ID, peerAddr, &cm.host)
		cm.connections[addrInfo.ID] = conn
	}

	// Attempts to connect to every bootstrapped peers.
	utils.LogInfo("Trying to create connections with peers...")
	wg := &sync.WaitGroup{}
	for _, peerAddr := range cm.bootstrapPeers {
		peerinfo, _ := peer.AddrInfoFromP2pAddr(peerAddr)
		wg.Add(1)

		go func() {
			defer wg.Done()

			// Retry 5 times at max.
			for i := 0; i < 5; i++ {
				if err := cm.host.Connect(ctx, *peerinfo); err != nil {
					utils.LogWarn(fmt.Sprintf("Error while connecting to node %q: %-v", peerinfo, err))
					time.Sleep(time.Second * 3)
				} else {
					utils.LogInfo("Connection established with bootstrap node: %q", *peerinfo)
					break
				}
			}
		}()
	}
	wg.Wait()
}

func (cm *DefaultConnectionManager) connectToPeer(peerAddr maddr.Multiaddr) error {
	// TODO: handle the case where connection fails.
	pi, err := peer.AddrInfoFromP2pAddr(peerAddr)
	if err != nil {
		return fmt.Errorf("fail to add peer: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), TimeoutConnecting)
	defer cancel()

	if err := cm.host.Connect(ctx, *pi); err != nil {
		return err
	}

	return nil
}

func (cm *DefaultConnectionManager) WriteToStream(pID peer.ID, protocolId protocol.ID, msg []byte) error {
	conn := cm.connections[pID]
	if conn == nil {
		return errors.New("pID not found")
	}

	err := conn.writeToStream(msg, protocolId)

	return err
}

func (cm *DefaultConnectionManager) initializeStatusManager() {
	peerIds := make([]peer.ID, 0, len(cm.bootstrapPeers))
	for _, peerAddr := range cm.bootstrapPeers {
		addrInfo, _ := peer.AddrInfoFromP2pAddr(peerAddr)
		peerIds = append(peerIds, addrInfo.ID)
	}

	cm.statusManager = NewStatusManager(peerIds, cm)
	cm.statusManager.Start()
}
