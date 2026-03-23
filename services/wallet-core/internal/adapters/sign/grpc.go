package sign

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	pb "wallet-saas-v2/services/wallet-core/internal/pb"
	"wallet-saas-v2/services/wallet-core/internal/ports"
)

type GRPCSign struct {
	client    pb.WalletServiceClient
	conn      *grpc.ClientConn
	authToken string
}

func NewGRPC(addr, authToken string) (*GRPCSign, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &GRPCSign{
		client:    pb.NewWalletServiceClient(conn),
		conn:      conn,
		authToken: strings.TrimSpace(authToken),
	}, nil
}

func (s *GRPCSign) Close() error {
	if s == nil || s.conn == nil {
		return nil
	}
	return s.conn.Close()
}

func (s *GRPCSign) SignMessage(ctx context.Context, signType, keyID, messageHash string) (string, error) {
	resp, err := s.client.SignMessage(s.attachAuth(ctx), &pb.SignMessageRequest{
		KeyId:       keyID,
		SignType:    signType,
		MessageHash: messageHash,
	})
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(resp.GetSignature()) == "" {
		return "", fmt.Errorf("empty signature")
	}
	return resp.GetSignature(), nil
}

func (s *GRPCSign) DeriveKey(ctx context.Context, signType, keyID string) (ports.DerivedKey, error) {
	resp, err := s.client.DeriveKey(s.attachAuth(ctx), &pb.DeriveKeyRequest{
		KeyId:    keyID,
		SignType: signType,
	})
	if err != nil {
		return ports.DerivedKey{}, err
	}
	if resp.GetPublicKey() == nil || strings.TrimSpace(resp.GetPublicKey().GetCompressedHex()) == "" {
		return ports.DerivedKey{}, fmt.Errorf("empty derived key response")
	}

	out := ports.DerivedKey{
		KeyID:                     keyID,
		PublicKey:                 resp.GetPublicKey().GetCompressedHex(),
		AlternatePublicKey:        resp.GetPublicKey().GetUncompressedHex(),
		DerivationPath:            resp.GetDerivationPath(),
		PublicDerivationSupported: resp.GetPublicDerivationSupported(),
		AccountChainCode:          resp.GetAccountChainCode(),
		AccountDerivationPath:     resp.GetAccountDerivationPath(),
		CustodyScheme:             resp.GetCustodyScheme(),
	}
	if resp.GetAccountPublicKey() != nil {
		out.AccountPublicKey = resp.GetAccountPublicKey().GetCompressedHex()
		out.AccountAlternatePublicKey = resp.GetAccountPublicKey().GetUncompressedHex()
	}
	return out, nil
}

func (s *GRPCSign) attachAuth(ctx context.Context) context.Context {
	if strings.TrimSpace(s.authToken) == "" {
		return ctx
	}
	return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+s.authToken)
}
