package server

type TssApi struct {
}

func (api *TssApi) GetVersion() string {
	return "1"
}
