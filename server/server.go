package server

import (
	"fmt"
	"net"
	"net/http"

	"github.com/sisu-network/tuktuk/utils"

	"github.com/ethereum/go-ethereum/rpc"
)

type Server struct {
	handler       *rpc.Server
	listenAddress string
}

func NewServer(handler *rpc.Server, host string, port uint16) *Server {
	return &Server{
		handler:       handler,
		listenAddress: fmt.Sprintf("%s:%d", host, port),
	}
}

func (s *Server) Run() {
	listener, err := net.Listen("tcp", s.listenAddress)
	if err != nil {
		panic(err)
	}

	srv := &http.Server{Handler: s.handler}
	utils.LogInfo("Running server at", s.listenAddress)
	srv.Serve(listener)
}
