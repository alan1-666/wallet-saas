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
		ConsumerToken: keyID,
		Type:          signType,
		PublicKey:     keyID,
		MessageHash:   messageHash,
	})
	if err != nil {
		return "", err
	}
	if resp.GetCode() != "1" {
		return "", fmt.Errorf("sign service error: %s", resp.GetMsg())
	}
	return resp.GetSignature(), nil
}

func (s *GRPCSign) DeriveKey(ctx context.Context, signType, keyID string) (ports.DerivedKey, error) {
	if signType == "" {
		signType = "ecdsa"
	}
	resp, err := s.client.ExportPublicKeyList(ctx, &pb.ExportPublicKeyRequest{
		ConsumerToken: keyID,
		Type:          signType,
		Number:        1,
	})
	if err != nil {
		return ports.DerivedKey{}, err
	}
	if resp.GetCode() != "1" {
		return ports.DerivedKey{}, fmt.Errorf("sign service error: %s", resp.GetMsg())
	}
	if len(resp.GetPublicKey()) == 0 {
		return ports.DerivedKey{}, fmt.Errorf("empty derived key response")
	}
	item := resp.GetPublicKey()[0]
	return ports.DerivedKey{
		KeyID:              keyID,
		PublicKey:          item.GetCompressPubkey(),
		AlternatePublicKey: item.GetDecompressPubkey(),
	}, nil
}
