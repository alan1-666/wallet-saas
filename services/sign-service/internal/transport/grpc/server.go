package grpctransport

import (
	"context"
	"fmt"
	"net"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	"wallet-saas-v2/services/sign-service/internal/config"
	"wallet-saas-v2/services/sign-service/internal/custody"
	"wallet-saas-v2/services/sign-service/internal/hd"
	pb "wallet-saas-v2/services/sign-service/internal/pb"
	"wallet-saas-v2/services/sign-service/internal/policy"
)

const maxRecvMessageSize = 1024 * 1024 * 10

type GRPCServer struct {
	cfg     config.Config
	custody custody.Provider
	policy  *policy.Engine

	pb.UnimplementedWalletServiceServer
}

func New(cfg config.Config, provider custody.Provider, policyEngine *policy.Engine) *GRPCServer {
	return &GRPCServer{cfg: cfg, custody: provider, policy: policyEngine}
}

func (s *GRPCServer) Start(ctx context.Context) error {
	_ = ctx
	addr := fmt.Sprintf("%s:%d", s.cfg.GRPCHost, s.cfg.GRPCPort)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	gs := grpc.NewServer(grpc.MaxRecvMsgSize(maxRecvMessageSize))
	pb.RegisterWalletServiceServer(gs, s)
	reflection.Register(gs)
	return gs.Serve(listener)
}

func (s *GRPCServer) GetSupportSignWay(_ context.Context, req *pb.GetSupportSignWayRequest) (*pb.GetSupportSignWayResponse, error) {
	switch normalizeSignType(req.GetSignType()) {
	case "ecdsa", "eddsa":
		return &pb.GetSupportSignWayResponse{Support: true, Message: "supported"}, nil
	default:
		return &pb.GetSupportSignWayResponse{Support: false, Message: "unsupported sign type"}, nil
	}
}

func (s *GRPCServer) DeriveKey(ctx context.Context, req *pb.DeriveKeyRequest) (*pb.DeriveKeyResponse, error) {
	decision, err := s.policy.Authorize(ctx, "derive", req.GetSignType(), req.GetKeyId())
	if err != nil {
		return nil, err
	}
	ref, err := hd.ParseKeyID(req.GetKeyId())
	if err != nil {
		return nil, err
	}
	derived, err := s.custody.DeriveKey(decision.TenantID, ref)
	if err != nil {
		return nil, err
	}
	resp := &pb.DeriveKeyResponse{
		KeyId:                     derived.KeyID,
		SignType:                  ref.SignType,
		DerivationPath:            derived.DerivationPath,
		PublicKey:                 &pb.PublicKey{CompressedHex: derived.PublicKeyHex, UncompressedHex: derived.AlternatePublicKey},
		PublicDerivationSupported: derived.PublicDerivationSupported,
		CustodyScheme:             s.custody.CustodyScheme(),
	}
	if derived.AccountPublicKeyHex != "" || derived.AccountAlternatePublicKey != "" {
		resp.AccountPublicKey = &pb.PublicKey{
			CompressedHex:   derived.AccountPublicKeyHex,
			UncompressedHex: derived.AccountAlternatePublicKey,
		}
		resp.AccountChainCode = derived.AccountChainCodeHex
		resp.AccountDerivationPath = derived.AccountDerivationPath
	}
	return resp, nil
}

func (s *GRPCServer) SignMessage(ctx context.Context, req *pb.SignMessageRequest) (*pb.SignMessageResponse, error) {
	decision, err := s.policy.Authorize(ctx, "sign", req.GetSignType(), req.GetKeyId())
	if err != nil {
		return nil, err
	}
	msgHash := strings.TrimSpace(req.GetMessageHash())
	if msgHash == "" {
		return nil, status.Error(codes.InvalidArgument, "message_hash is required")
	}
	msgHash = strings.TrimPrefix(msgHash, "0x")
	if len(msgHash) != 64 {
		return nil, status.Errorf(codes.InvalidArgument, "message_hash must be 32 bytes hex (got %d hex chars)", len(msgHash))
	}
	ref, err := hd.ParseKeyID(req.GetKeyId())
	if err != nil {
		return nil, err
	}
	signature, err := s.custody.SignMessage(decision.TenantID, ref, msgHash)
	if err != nil {
		return nil, err
	}
	return &pb.SignMessageResponse{
		Signature:     signature,
		Message:       "signed",
		CustodyScheme: s.custody.CustodyScheme(),
	}, nil
}

func normalizeSignType(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "ecdsa":
		return "ecdsa"
	case "eddsa", "ed25519":
		return "eddsa"
	default:
		return ""
	}
}
