package client

import (
	"context"
	"fmt"
	"strings"
	"time"

	pb "wallet-saas-v2/services/scan-utxo-service/internal/pb/chaingateway"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type ChainGateway struct {
	conn    *grpc.ClientConn
	client  pb.ChainGatewayServiceClient
	timeout time.Duration
}

type IncomingTransfer struct {
	TxHash        string
	FromAddress   string
	ToAddress     string
	Amount        string
	Confirmations int64
	Index         int64
	Status        string
}

type TxFinality struct {
	TxHash        string
	Confirmations int64
	Status        string
	Found         bool
}

func NewChainGateway(addr string, timeout time.Duration) (*ChainGateway, error) {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &ChainGateway{
		conn:    conn,
		client:  pb.NewChainGatewayServiceClient(conn),
		timeout: timeout,
	}, nil
}

func (c *ChainGateway) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

func (c *ChainGateway) ListIncomingTransfers(ctx context.Context, model, chain, coin, network, address, cursor string, pageSize int) ([]IncomingTransfer, string, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	network = strings.TrimSpace(network)
	if network == "" {
		return nil, "", fmt.Errorf("network is required")
	}
	resp, err := c.client.ListIncomingTransfers(ctx, &pb.ListIncomingTransfersRequest{
		Model:    model,
		Chain:    chain,
		Coin:     coin,
		Network:  network,
		Address:  address,
		Page:     1,
		PageSize: uint32(pageSize),
		Cursor:   cursor,
	})
	if err != nil {
		return nil, "", err
	}
	out := make([]IncomingTransfer, 0, len(resp.GetItems()))
	for _, item := range resp.Items {
		out = append(out, IncomingTransfer{
			TxHash:        item.GetTxHash(),
			FromAddress:   item.GetFromAddress(),
			ToAddress:     item.GetToAddress(),
			Index:         item.GetIndex(),
			Amount:        item.GetAmount(),
			Confirmations: item.GetConfirmations(),
			Status:        item.GetStatus(),
		})
	}
	return out, resp.GetNextCursor(), nil
}

func (c *ChainGateway) TxFinality(ctx context.Context, chain, coin, network, txHash string) (TxFinality, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	network = strings.TrimSpace(network)
	if network == "" {
		return TxFinality{}, fmt.Errorf("network is required")
	}
	out, err := c.client.GetTxFinality(ctx, &pb.TxFinalityRequest{
		Chain:   chain,
		Coin:    coin,
		Network: network,
		TxHash:  txHash,
	})
	if err != nil {
		return TxFinality{}, err
	}
	return TxFinality{
		TxHash:        out.GetTxHash(),
		Confirmations: out.GetConfirmations(),
		Status:        strings.ToUpper(strings.TrimSpace(out.GetStatus())),
		Found:         out.GetFound(),
	}, nil
}

func (c *ChainGateway) GetBalance(ctx context.Context, chain, coin, network, address string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	network = strings.TrimSpace(network)
	if network == "" {
		return "", fmt.Errorf("network is required")
	}
	out, err := c.client.GetBalance(ctx, &pb.BalanceRequest{
		Chain:   chain,
		Coin:    coin,
		Network: network,
		Address: address,
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out.GetBalance()), nil
}
