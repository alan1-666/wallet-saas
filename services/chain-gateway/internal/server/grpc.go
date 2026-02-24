package server

import (
	"context"

	pb "wallet-saas-v2/services/chain-gateway/internal/pb/chaingateway"
	"wallet-saas-v2/services/chain-gateway/internal/ports"
	"wallet-saas-v2/services/chain-gateway/internal/service"
)

type GRPCServer struct {
	pb.UnimplementedChainGatewayServiceServer
	Chain *service.ChainService
}

func NewGRPC(chain *service.ChainService) *GRPCServer {
	return &GRPCServer{Chain: chain}
}

func (s *GRPCServer) ConvertAddress(ctx context.Context, req *pb.ConvertAddressRequest) (*pb.ConvertAddressResponse, error) {
	address, err := s.Chain.ConvertAddress(ctx, req.GetChain(), req.GetType(), req.GetPublicKey())
	if err != nil {
		return nil, err
	}
	return &pb.ConvertAddressResponse{Address: address}, nil
}

func (s *GRPCServer) BuildUnsignedTx(ctx context.Context, req *pb.BuildUnsignedTxRequest) (*pb.BuildUnsignedTxResponse, error) {
	vin := make([]ports.TxVin, 0, len(req.GetVin()))
	for _, x := range req.GetVin() {
		vin = append(vin, ports.TxVin{Hash: x.GetHash(), Index: x.GetIndex(), Amount: x.GetAmount(), Address: x.GetAddress()})
	}
	vout := make([]ports.TxVout, 0, len(req.GetVout()))
	for _, x := range req.GetVout() {
		vout = append(vout, ports.TxVout{Address: x.GetAddress(), Amount: x.GetAmount(), Index: x.GetIndex()})
	}

	res, err := s.Chain.BuildUnsigned(ctx, service.BuildUnsignedInput{
		Chain:    req.GetChain(),
		Network:  req.GetNetwork(),
		Base64Tx: req.GetBase64Tx(),
		Fee:      req.GetFee(),
		Vin:      vin,
		Vout:     vout,
	})
	if err != nil {
		return nil, err
	}
	return &pb.BuildUnsignedTxResponse{UnsignedTx: res.UnsignedTx, SignHashes: res.SignHashes}, nil
}

func (s *GRPCServer) SendTx(ctx context.Context, req *pb.SendTxRequest) (*pb.SendTxResponse, error) {
	txHash, err := s.Chain.SendTx(ctx, service.SendTxInput{
		Chain:      req.GetChain(),
		Network:    req.GetNetwork(),
		Coin:       req.GetCoin(),
		RawTx:      req.GetRawTx(),
		UnsignedTx: req.GetUnsignedTx(),
		Signatures: req.GetSignatures(),
		PublicKeys: req.GetPublicKeys(),
	})
	if err != nil {
		return nil, err
	}
	return &pb.SendTxResponse{TxHash: txHash}, nil
}
