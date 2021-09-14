package client

import (
	"context"
	"errors"
	"time"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/sisu-network/dheart/types"
	"github.com/sisu-network/dheart/utils"
)

const (
	RETRY_TIME = 10 * time.Second
)

var (
	SISU_SERVER_NOT_CONNECTED = errors.New("Sisu server is not connected")
)

type Client interface {
	TryDial()
	PostKeygenResult(workId string)
	BroadcastKeygenResult(chain string, pubKeyBytes []byte, address string) error
	BroadcastKeySignResult(result *types.KeysignResult) error
}

// A client that connects to Sisu server
type DefaultClient struct {
	client    *rpc.Client
	url       string
	connected bool
}

func NewClient(url string) Client {
	return &DefaultClient{
		url: url,
	}
}

func (c *DefaultClient) TryDial() {
	utils.LogInfo("Trying to dial Sisu server")

	for {
		utils.LogInfo("Dialing...", c.url)
		c.client, _ = rpc.DialContext(context.Background(), c.url)
		if err := c.CheckHealth(); err == nil {
			c.connected = true
			break
		}

		time.Sleep(RETRY_TIME)
	}

	utils.LogInfo("Sisu server is connected")
}

func (c *DefaultClient) CheckHealth() error {
	var result interface{}
	err := c.client.CallContext(context.Background(), &result, "tss_checkHealth")
	if err != nil {
		utils.LogError("Cannot check dheart health, err = ", err)
		return err
	}

	return nil
}

// @Deprecated
func (c *DefaultClient) BroadcastKeygenResult(chain string, pubKeyBytes []byte, address string) error {
	utils.LogDebug("c.connected = ", c.connected)

	if !c.connected {
		return SISU_SERVER_NOT_CONNECTED
	}

	utils.LogDebug("Sending keygen result to sisu server")

	keygenResult := types.KeygenResult{
		Chain:       chain,
		Success:     true,
		PubKeyBytes: pubKeyBytes,
		Address:     address,
	}

	var r interface{}
	err := c.client.CallContext(context.Background(), &r, "tss_keygenResult", keygenResult)
	if err != nil {
		// TODO: Retry on failure.
		utils.LogError("Cannot post keygen result, err = ", err)
		return err
	}

	return nil
}

func (c *DefaultClient) PostKeygenResult(workId string) {
	// TODO: implement this.
}

func (c *DefaultClient) BroadcastKeySignResult(result *types.KeysignResult) error {
	var r interface{}
	err := c.client.CallContext(context.Background(), &r, "tss_keySignResult", result)
	if err != nil {
		// TODO: Retry on failure.
		utils.LogError("Cannot post keygen result, err = ", err)
		return err
	}

	return nil
}
