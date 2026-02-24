package app

import (
	"log"
	"net"
	"net/http"

	"wallet-saas-v2/services/chain-gateway/internal/adapters/evm"
	"wallet-saas-v2/services/chain-gateway/internal/adapters/utxo"
	"wallet-saas-v2/services/chain-gateway/internal/config"
	"wallet-saas-v2/services/chain-gateway/internal/dispatcher"
	"wallet-saas-v2/services/chain-gateway/internal/handler"
	pb "wallet-saas-v2/services/chain-gateway/internal/pb/chaingateway"
	"wallet-saas-v2/services/chain-gateway/internal/server"
	"wallet-saas-v2/services/chain-gateway/internal/service"

	accdispatcher "github.com/dapplink-labs/wallet-chain-account/chaindispatcher"
	accconfig "github.com/dapplink-labs/wallet-chain-account/config"
	utxodispatcher "github.com/dapplink-labs/wallet-chain-utxo/chaindispatcher"
	utxoconfig "github.com/dapplink-labs/wallet-chain-utxo/config"

	"google.golang.org/grpc"
)

func Run() error {
	cfg := config.Load()

	accCfg, err := accconfig.New(cfg.AccountCfgPath)
	if err != nil {
		return err
	}
	accountDispatcher, err := accdispatcher.New(accCfg)
	if err != nil {
		return err
	}

	utxoCfg, err := utxoconfig.New(cfg.UtxoCfgPath)
	if err != nil {
		return err
	}
	utxoDispatcher, err := utxodispatcher.New(utxoCfg)
	if err != nil {
		return err
	}

	router := &dispatcher.Router{
		Account: &evm.Adapter{Dispatcher: accountDispatcher},
		UTXO:    &utxo.Adapter{Dispatcher: utxoDispatcher},
	}
	chainSvc := &service.ChainService{Router: router}

	chainHandler := &handler.ChainHandler{Chain: chainSvc}
	go func() {
		if err := http.ListenAndServe(cfg.HTTPAddr, server.NewMux(chainHandler)); err != nil {
			log.Printf("chain-gateway http stopped: %v", err)
		}
	}()

	lis, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		return err
	}
	grpcSrv := grpc.NewServer()
	pb.RegisterChainGatewayServiceServer(grpcSrv, server.NewGRPC(chainSvc))
	return grpcSrv.Serve(lis)
}
