package mock

import (
	"github.com/sisu-network/dheart/types"
	"github.com/sisu-network/lib/log"
)

type ApiHandler struct {
	pingCh    chan string
	keygenCh  chan *types.KeygenResult
	keysignCh chan *types.KeysignResult
}

func NewApi(keygenCh chan *types.KeygenResult, keysignCh chan *types.KeysignResult, pingCh chan string) *ApiHandler {
	return &ApiHandler{
		pingCh:    pingCh,
		keygenCh:  keygenCh,
		keysignCh: keysignCh,
	}
}

func (a *ApiHandler) Version() string {
	return "1.0"
}

// Empty function for checking health only.
func (api *ApiHandler) Ping(source string) {
	api.pingCh <- source
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
