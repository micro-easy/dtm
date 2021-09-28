// Code generated by goctl. DO NOT EDIT!
// Source: dtm.proto

package main

import (
	"flag"
	"fmt"

	//_ "net/http/pprof"

	"github.com/micro-easy/dtm/cmd/dtmrpc/dtm"
	"github.com/micro-easy/dtm/cmd/dtmrpc/internal/config"
	"github.com/micro-easy/dtm/cmd/dtmrpc/internal/logic"
	"github.com/micro-easy/dtm/cmd/dtmrpc/internal/server"
	"github.com/micro-easy/dtm/cmd/dtmrpc/internal/svc"

	"github.com/micro-easy/go-zero/core/conf"
	"github.com/micro-easy/go-zero/core/proc"
	"github.com/micro-easy/go-zero/zrpc"
	"google.golang.org/grpc"
)

var configFile = flag.String("f", "etc/dtmrpc.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)
	ctx := svc.NewServiceContext(c)
	logic.InitTasks(ctx)

	srv := server.NewDtmServer(ctx)

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		dtm.RegisterDtmServer(grpcServer, srv)
	})

	// 关闭钩子
	waitForCalled := proc.AddShutdownListener(func() {
		svc.Stop(ctx)
	})
	defer waitForCalled()

	defer s.Stop()

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	s.Start()
}
