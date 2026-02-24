package base

import (
	"github.com/ethereum/go-ethereum/log"

	"github.com/btcsuite/btcd/rpcclient"
)

type BaseClient struct {
	*rpcclient.Client
	compressed bool
}

func NewBaseClient(RpcUrl, RpcUser, RpcPass string) (*BaseClient, error) {
	client, err := rpcclient.New(&rpcclient.ConnConfig{
		Host:         RpcUrl,
		User:         RpcUser,
		Pass:         RpcPass,
		HTTPPostMode: true,
		DisableTLS:   true,
	}, nil)
	if err != nil {
		log.Error("new bitcoin rpc client fail", "err", err)
		return nil, err
	}
	return &BaseClient{
		Client:     client,
		compressed: true,
	}, nil
}
