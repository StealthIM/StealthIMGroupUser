package main

import (
	"StealthIMGroupUser/config"
	"StealthIMGroupUser/gateway"
	"StealthIMGroupUser/grpc"
	"StealthIMGroupUser/user"
	"log"
)

func main() {
	cfg := config.ReadConf()
	log.Printf("Start server [%v]\n", config.Version)
	log.Printf("+ GRPC\n")
	log.Printf("    Host: %s\n", cfg.GRPCProxy.Host)
	log.Printf("    Port: %d\n", cfg.GRPCProxy.Port)
	log.Printf("+ DBGateway\n")
	log.Printf("    Host: %s\n", cfg.DBGateway.Host)
	log.Printf("    Port: %d\n", cfg.DBGateway.Port)
	log.Printf("    ConnNum: %d\n", cfg.DBGateway.ConnNum)
	log.Printf("+ User\n")
	log.Printf("    Host: %s\n", cfg.User.Host)
	log.Printf("    Port: %d\n", cfg.User.Port)
	log.Printf("    ConnNum: %d\n", cfg.User.ConnNum)
	log.Printf("+ Session\n")
	log.Printf("    Host: %s\n", cfg.Session.Host)
	log.Printf("    Port: %d\n", cfg.Session.Port)
	log.Printf("    ConnNum: %d\n", cfg.Session.ConnNum)

	// 启动 DBGateway
	go gateway.InitConns()
	go user.InitConns()

	// 启动 GRPC 服务
	grpc.Start(cfg)
}
