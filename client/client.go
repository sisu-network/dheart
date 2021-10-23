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
	RetryTime = 10 * time.Second
)

var (
	ErrSisuServerNotConnected = errors.New("Sisu server is not connected")
)

type Client interface {
	TryDial()
	PostKeygenResult(result *types.KeygenResult) error
	PostPresignResult(result *types.PresignResult) error
	PostKeysignResult(result *types.KeysignResult) error
}

// A client that connects to Sisu server
type DefaultClient struct {
	client *rpc.Client
	url    string
}

func NewClient(url string) Client {
	return &DefaultClient{
		url: url,
	}
}

func (c *DefaultClient) TryDial() {
	utils.LogInfo("Trying to dial Sisu server, url = ", c.url)

	for {
		utils.LogInfo("Dialing...", c.url)
		var err error
		c.client, err = rpc.DialContext(context.Background(), c.url)
		if err == nil {
			if err := c.CheckHealth(); err == nil {
				break
			}
		} else {
			utils.LogError("Cannot dial, err = ", err)
		}

		time.Sleep(RetryTime)
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

func (c *DefaultClient) PostKeygenResult(result *types.KeygenResult) error {
	utils.LogDebug("Sending keygen result to sisu server")

	var r interface{}
	err := c.client.CallContext(context.Background(), &r, "tss_keygenResult", result)
	if err != nil {
		// TODO: Retry on failure.
		utils.LogError("Cannot post keygen result, err = ", err)
		return err
	}

	return nil
}

func (c *DefaultClient) PostPresignResult(result *types.PresignResult) error {
	utils.LogDebug("Sending presign result to sisu server")

	var r interface{}
	err := c.client.CallContext(context.Background(), &r, "tss_presignResult", result)
	if err != nil {
		// TODO: Retry on failure.
		utils.LogError("Cannot post presign result, err = ", err)
		return err
	}

	return nil
}

func (c *DefaultClient) PostKeysignResult(result *types.KeysignResult) error {
	utils.LogDebug("Sending keysign result to sisu server")

	var r interface{}
	err := c.client.CallContext(context.Background(), &r, "tss_keysignResult", result)
	if err != nil {
		// TODO: Retry on failure.
		utils.LogError("Cannot post keysign result, err = ", err)
		return err
	}

	return nil
}
