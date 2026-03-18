package chain

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

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
	if params.Chain == "" || params.Network == "" {
		return ports.BuildUnsignedResult{}, fmt.Errorf("chain and network are required")
	}
	base64Tx, err := buildBase64Tx(params)
	if err != nil {
		return ports.BuildUnsignedResult{}, err
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
		Network:  params.Network,
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

type accountTxPayload struct {
	ContractAddress string `json:"contract_address,omitempty"`
	FromAddress     string `json:"from_address"`
	ToAddress       string `json:"to_address"`
	Value           string `json:"value"`
	AmountUnit      string `json:"amount_unit,omitempty"`
	TokenDecimals   uint32 `json:"token_decimals,omitempty"`
}

func buildBase64Tx(params ports.BuildUnsignedParams) (string, error) {
	if strings.TrimSpace(params.Base64Tx) != "" {
		return strings.TrimSpace(params.Base64Tx), nil
	}
	if strings.TrimSpace(params.Chain) == "" {
		return "", fmt.Errorf("chain is required")
	}
	if strings.TrimSpace(params.From) == "" || strings.TrimSpace(params.To) == "" || strings.TrimSpace(params.Amount) == "" {
		return "", fmt.Errorf("from/to/amount are required")
	}
	if strings.EqualFold(strings.TrimSpace(params.Chain), "solana") && (strings.TrimSpace(params.ContractAddress) != "" || strings.TrimSpace(params.AmountUnit) != "" || params.TokenDecimals > 0) {
		payload := accountTxPayload{
			ContractAddress: strings.TrimSpace(params.ContractAddress),
			FromAddress:     strings.TrimSpace(params.From),
			ToAddress:       strings.TrimSpace(params.To),
			Value:           strings.TrimSpace(params.Amount),
			AmountUnit:      strings.TrimSpace(params.AmountUnit),
			TokenDecimals:   params.TokenDecimals,
		}
		raw, err := json.Marshal(payload)
		if err != nil {
			return "", err
		}
		return base64.StdEncoding.EncodeToString(raw), nil
	}
	raw := params.Chain + ":" + params.From + ":" + params.To + ":" + params.Amount
	return base64.StdEncoding.EncodeToString([]byte(raw)), nil
}

func (g *GRPCChain) Broadcast(ctx context.Context, params ports.BroadcastParams) (string, error) {
	if params.Chain == "" || params.Network == "" {
		return "", fmt.Errorf("chain and network are required")
	}
	resp, err := g.client.SendTx(ctx, &pb.SendTxRequest{
		Chain:      params.Chain,
		Network:    params.Network,
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

func (g *GRPCChain) GetBalance(ctx context.Context, chain, coin, network, address, contractAddress string) (ports.ChainBalance, error) {
	if chain == "" || network == "" {
		return ports.ChainBalance{}, fmt.Errorf("chain and network are required")
	}
	resp, err := g.client.GetBalance(ctx, &pb.BalanceRequest{
		Chain:           chain,
		Coin:            coin,
		Network:         network,
		Address:         address,
		ContractAddress: contractAddress,
	})
	if err != nil {
		return ports.ChainBalance{}, err
	}
	return ports.ChainBalance{
		Balance:  resp.GetBalance(),
		Network:  resp.GetNetwork(),
		Sequence: resp.GetSequence(),
	}, nil
}

func (g *GRPCChain) ConvertAddress(ctx context.Context, chain, network, addrType, publicKey string) (string, error) {
	if chain == "" || network == "" {
		return "", fmt.Errorf("chain and network are required")
	}
	resp, err := g.client.ConvertAddress(ctx, &pb.ConvertAddressRequest{
		Chain:     chain,
		Network:   network,
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
