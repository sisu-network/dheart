package mock

import (
	"github.com/sisu-network/dheart/types"
	"github.com/sisu-network/lib/log"
)

type ApiHandler struct {
	keygenCh  chan *types.KeygenResult
	keysignCh chan *types.KeysignResult
}

func NewApi(keygenCh chan *types.KeygenResult, keysignCh chan *types.KeysignResult) *ApiHandler {
	return &ApiHandler{
		keygenCh:  keygenCh,
		keysignCh: keysignCh,
	}
}

func (a *ApiHandler) Version() string {
	return "1.0"
}

// Empty function for checking health only.
func (api *ApiHandler) Ping(source string) {
	// Do nothing.
}

func (a *ApiHandler) KeygenResult(result *types.KeygenResult) bool {
	log.Info("There is a Keygen Result")
	log.Info("Success = ", result.Success)

	if a.keygenCh != nil {
		a.keygenCh <- result
	}

	return true
}

func (a *ApiHandler) KeysignResult(result *types.KeysignResult) {
	log.Info("There is keysign result")

	if a.keysignCh != nil {
		a.keysignCh <- result
	}
}
