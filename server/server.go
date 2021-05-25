package server

import (
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/sisu-network/tuktuk/rpc"
	"github.com/sisu-network/tuktuk/utils"
)

type Server struct {
}

func (s *Server) Run(host string, port uint16) {
	handler := rpc.NewServer(time.Second * 10)
	handler.RegisterName("tss", &TssApi{})

	listenAddress := fmt.Sprintf("%s:%d", host, port)

	listener, err := net.Listen("tcp", listenAddress)
	if err != nil {
		panic(err)
	}

	srv := &http.Server{Handler: handler}
	utils.LogInfo("Running server...")
	srv.Serve(listener)
}
