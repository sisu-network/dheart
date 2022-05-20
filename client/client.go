package client

import (
	"context"
	"errors"
	"time"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/sisu-network/dheart/types"
	"github.com/sisu-network/lib/log"
)

const (
	ConnectionRetryTime = 10 * time.Second
	PostRetryTime       = 2 * time.Second
	MaxRetry            = 5
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
	log.Info("Trying to dial Sisu server, url = ", c.url)

	for {
		log.Info("Dialing...", c.url)
		var err error
		c.client, err = rpc.DialContext(context.Background(), c.url)
		if err == nil {
			if err := c.Ping("dheart"); err == nil {
				break
			}
		} else {
			log.Error("Cannot dial, err = ", err)
		}

		time.Sleep(ConnectionRetryTime)
	}

	log.Info("Sisu server is connected")
}

func (c *DefaultClient) Ping(source string) error {
	var result interface{}
	err := c.client.CallContext(context.Background(), &result, "tss_ping", source)
	if err != nil {
		log.Error("Cannot ping sisu, err = ", err)
		return err
	}

	return nil
}

func (c *DefaultClient) PostKeygenResult(result *types.KeygenResult) error {
	log.Debug("Sending keygen result to sisu server")

	var r interface{}
	var err error
	for i := 0; i < MaxRetry; i++ {
		err = c.client.CallContext(context.Background(), &r, "tss_keygenResult", result)
		if err == nil {
			return nil
		}
	}

	if err != nil {
		log.Error("Cannot post keygen result, err = ", err)
		return err
	}

	return nil
}

func (c *DefaultClient) PostPresignResult(result *types.PresignResult) error {
	log.Debug("Sending presign result to sisu server")

	var r interface{}
	var err error
	for i := 0; i < MaxRetry; i++ {
		err = c.client.CallContext(context.Background(), &r, "tss_presignResult", result)
		if err == nil {
			return nil
		}
	}

	if err != nil {
		log.Error("Cannot post presign result, err = ", err)
		return err
	}

	return nil
}

func (c *DefaultClient) PostKeysignResult(result *types.KeysignResult) error {
	log.Debug("Sending keysign result to sisu server")

	var r interface{}
	var err error
	for i := 0; i < MaxRetry; i++ {
		err = c.client.CallContext(context.Background(), &r, "tss_keysignResult", result)
		if err == nil {
			return nil
		}
	}

	if err != nil {
		log.Error("Cannot post keysign result, err = ", err)
		return err
	}

	return nil
}
