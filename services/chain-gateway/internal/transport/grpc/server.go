package grpctransport

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
	address, err := s.Chain.ConvertAddress(ctx, req.GetChain(), req.GetNetwork(), req.GetType(), req.GetPublicKey())
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

func (s *GRPCServer) ListIncomingTransfers(ctx context.Context, req *pb.ListIncomingTransfersRequest) (*pb.ListIncomingTransfersResponse, error) {
	out, err := s.Chain.ListIncomingTransfers(ctx, ports.IncomingTransferInput{
		Model:           ports.ChainModel(req.GetModel()),
		Chain:           req.GetChain(),
		Coin:            req.GetCoin(),
		Network:         req.GetNetwork(),
		Address:         req.GetAddress(),
		Page:            req.GetPage(),
		PageSize:        req.GetPageSize(),
		Cursor:          req.GetCursor(),
		ContractAddress: req.GetContractAddress(),
	})
	if err != nil {
		return nil, err
	}
	items := make([]*pb.IncomingTransfer, 0, len(out.Items))
	for _, it := range out.Items {
		items = append(items, &pb.IncomingTransfer{
			TxHash:          it.TxHash,
			FromAddress:     it.FromAddress,
			ToAddress:       it.ToAddress,
			Amount:          it.Amount,
			Confirmations:   it.Confirmations,
			Index:           it.Index,
			Status:          it.Status,
			ContractAddress: it.ContractAddress,
		})
	}
	pbBlocks := make([]*pb.BlockMeta, 0, len(out.Blocks))
	for _, b := range out.Blocks {
		pbBlocks = append(pbBlocks, &pb.BlockMeta{
			Number:     b.Number,
			Hash:       b.Hash,
			ParentHash: b.ParentHash,
		})
	}
	return &pb.ListIncomingTransfersResponse{
		Items:      items,
		NextCursor: out.NextCursor,
		Blocks:     pbBlocks,
	}, nil
}

func (s *GRPCServer) GetTxFinality(ctx context.Context, req *pb.TxFinalityRequest) (*pb.TxFinalityResponse, error) {
	out, err := s.Chain.GetTxFinality(ctx, ports.TxFinalityInput{
		Chain:   req.GetChain(),
		Coin:    req.GetCoin(),
		Network: req.GetNetwork(),
		TxHash:  req.GetTxHash(),
	})
	if err != nil {
		return nil, err
	}
	return &pb.TxFinalityResponse{
		TxHash:        out.TxHash,
		Confirmations: out.Confirmations,
		Status:        out.Status,
		Found:         out.Found,
	}, nil
}

func (s *GRPCServer) GetBalance(ctx context.Context, req *pb.BalanceRequest) (*pb.BalanceResponse, error) {
	out, err := s.Chain.GetBalance(ctx, ports.BalanceInput{
		Chain:           req.GetChain(),
		Coin:            req.GetCoin(),
		Network:         req.GetNetwork(),
		Address:         req.GetAddress(),
		ContractAddress: req.GetContractAddress(),
	})
	if err != nil {
		return nil, err
	}
	return &pb.BalanceResponse{
		Balance:  out.Balance,
		Network:  out.Network,
		Sequence: out.Sequence,
	}, nil
}
