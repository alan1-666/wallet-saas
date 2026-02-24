package chain

import (
	"context"
	"encoding/base64"
	"fmt"

	pb "wallet-saas-v2/services/wallet-core/internal/pb/chaingateway"
	"wallet-saas-v2/services/wallet-core/internal/ports"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type GRPCChain struct {
	conn   *grpc.ClientConn
	client pb.ChainGatewayServiceClient
}

func NewGRPC(addr string) (*GRPCChain, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &GRPCChain{conn: conn, client: pb.NewChainGatewayServiceClient(conn)}, nil
}

func (g *GRPCChain) Close() error {
	if g == nil || g.conn == nil {
		return nil
	}
	return g.conn.Close()
}

func (g *GRPCChain) BuildUnsignedTx(ctx context.Context, params ports.BuildUnsignedParams) (ports.BuildUnsignedResult, error) {
	network := params.Network
	if network == "" {
		network = "mainnet"
	}
	base64Tx := params.Base64Tx
	if base64Tx == "" {
		raw := params.Chain + ":" + params.From + ":" + params.To + ":" + params.Amount
		base64Tx = base64.StdEncoding.EncodeToString([]byte(raw))
	}
	vin := make([]*pb.TxVin, 0, len(params.Vin))
	for _, x := range params.Vin {
		vin = append(vin, &pb.TxVin{Hash: x.Hash, Index: x.Index, Amount: x.Amount, Address: x.Address})
	}
	vout := make([]*pb.TxVout, 0, len(params.Vout))
	for _, x := range params.Vout {
		vout = append(vout, &pb.TxVout{Address: x.Address, Amount: x.Amount, Index: x.Index})
	}

	resp, err := g.client.BuildUnsignedTx(ctx, &pb.BuildUnsignedTxRequest{
		Chain:    params.Chain,
		Network:  network,
		Coin:     params.Coin,
		Base64Tx: base64Tx,
		Fee:      params.Fee,
		Vin:      vin,
		Vout:     vout,
	})
	if err != nil {
		return ports.BuildUnsignedResult{}, err
	}
	if resp.GetUnsignedTx() == "" {
		return ports.BuildUnsignedResult{}, fmt.Errorf("empty unsigned tx")
	}
	return ports.BuildUnsignedResult{UnsignedTx: resp.GetUnsignedTx(), SignHashes: resp.GetSignHashes()}, nil
}

func (g *GRPCChain) Broadcast(ctx context.Context, params ports.BroadcastParams) (string, error) {
	network := params.Network
	if network == "" {
		network = "mainnet"
	}
	resp, err := g.client.SendTx(ctx, &pb.SendTxRequest{
		Chain:      params.Chain,
		Network:    network,
		Coin:       params.Coin,
		RawTx:      params.RawTx,
		UnsignedTx: params.UnsignedTx,
		Signatures: params.Signatures,
		PublicKeys: params.PublicKeys,
	})
	if err != nil {
		return "", err
	}
	if resp.GetTxHash() == "" {
		return "", fmt.Errorf("empty tx hash")
	}
	return resp.GetTxHash(), nil
}

func (g *GRPCChain) ConvertAddress(ctx context.Context, chain, addrType, publicKey string) (string, error) {
	resp, err := g.client.ConvertAddress(ctx, &pb.ConvertAddressRequest{
		Chain:     chain,
		Type:      addrType,
		PublicKey: publicKey,
	})
	if err != nil {
		return "", err
	}
	if resp.GetAddress() == "" {
		return "", fmt.Errorf("empty address")
	}
	return resp.GetAddress(), nil
}
