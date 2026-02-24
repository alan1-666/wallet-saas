package sign

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "wallet-saas-v2/services/wallet-core/internal/pb"
	"wallet-saas-v2/services/wallet-core/internal/ports"
)

type GRPCSign struct {
	client pb.WalletServiceClient
	conn   *grpc.ClientConn
}

func NewGRPC(addr string) (*GRPCSign, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &GRPCSign{client: pb.NewWalletServiceClient(conn), conn: conn}, nil
}

func (s *GRPCSign) Close() error {
	if s == nil || s.conn == nil {
		return nil
	}
	return s.conn.Close()
}

func (s *GRPCSign) SignMessage(ctx context.Context, signType, keyID, messageHash string) (string, error) {
	if signType == "" {
		signType = "eddsa"
	}
	resp, err := s.client.SignTxMessage(ctx, &pb.SignTxMessageRequest{
		Type:        signType,
		PublicKey:   keyID,
		MessageHash: messageHash,
	})
	if err != nil {
		return "", err
	}
	if resp.GetCode() != "1" {
		return "", fmt.Errorf("sign service error: %s", resp.GetMsg())
	}
	return resp.GetSignature(), nil
}

func (s *GRPCSign) ExportPublicKeys(ctx context.Context, signType string, number int32) ([]ports.PublicKeyPair, error) {
	if signType == "" {
		signType = "ecdsa"
	}
	if number <= 0 {
		number = 1
	}
	resp, err := s.client.ExportPublicKeyList(ctx, &pb.ExportPublicKeyRequest{
		Type:   signType,
		Number: uint64(number),
	})
	if err != nil {
		return nil, err
	}
	if resp.GetCode() != "1" {
		return nil, fmt.Errorf("sign service error: %s", resp.GetMsg())
	}
	out := make([]ports.PublicKeyPair, 0, len(resp.GetPublicKey()))
	for _, item := range resp.GetPublicKey() {
		out = append(out, ports.PublicKeyPair{
			CompressPubkey:   item.GetCompressPubkey(),
			DecompressPubkey: item.GetDecompressPubkey(),
		})
	}
	return out, nil
}
