package server

import (
	"context"
	"fmt"
	"net"
	"strconv"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"wallet-saas-v2/services/sign-service/internal/config"
	crypto2 "wallet-saas-v2/services/sign-service/internal/crypto"
	"wallet-saas-v2/services/sign-service/internal/keystore"
	pb "wallet-saas-v2/services/sign-service/internal/pb"
)

const maxRecvMessageSize = 1024 * 1024 * 300

type GRPCServer struct {
	cfg   config.Config
	store *keystore.Keys

	pb.UnimplementedWalletServiceServer
}

func New(cfg config.Config, store *keystore.Keys) *GRPCServer {
	return &GRPCServer{cfg: cfg, store: store}
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

func (s *GRPCServer) GetSupportSignWay(_ context.Context, req *pb.SupportSignWayRequest) (*pb.SupportSignWayResponse, error) {
	supported := req.GetType() == "ecdsa" || req.GetType() == "eddsa"
	if !supported {
		return &pb.SupportSignWayResponse{Code: "0", Msg: "Do not support this sign way", Support: false}, nil
	}
	return &pb.SupportSignWayResponse{Code: "1", Msg: "Support this sign way", Support: true}, nil
}

func (s *GRPCServer) ExportPublicKeyList(_ context.Context, req *pb.ExportPublicKeyRequest) (*pb.ExportPublicKeyResponse, error) {
	if req.GetNumber() == 0 {
		return &pb.ExportPublicKeyResponse{Code: "1", Msg: "no key requested"}, nil
	}
	if req.GetNumber() > 10000 {
		return &pb.ExportPublicKeyResponse{Code: "0", Msg: "Number must be less than 10000"}, nil
	}

	count := int(req.GetNumber())
	keyList := make([]keystore.Key, 0, count)
	ret := make([]*pb.PublicKey, 0, count)

	for i := 0; i < count; i++ {
		var priKeyStr, pubKeyStr, decPubkeyStr string
		var err error

		switch req.GetType() {
		case "ecdsa":
			priKeyStr, pubKeyStr, decPubkeyStr, err = crypto2.CreateECDSAKeyPair()
		case "eddsa":
			priKeyStr, pubKeyStr, err = crypto2.CreateEdDSAKeyPair()
			decPubkeyStr = pubKeyStr
		default:
			return nil, fmt.Errorf("unsupported key type")
		}
		if err != nil {
			return nil, err
		}

		keyList = append(keyList, keystore.Key{PrivateKey: priKeyStr, CompressPubkey: pubKeyStr})
		ret = append(ret, &pb.PublicKey{CompressPubkey: pubKeyStr, DecompressPubkey: decPubkeyStr})
	}

	if ok := s.store.StoreKeys(keyList); !ok {
		return nil, fmt.Errorf("store keys failed")
	}

	return &pb.ExportPublicKeyResponse{
		Code:      strconv.Itoa(1),
		Msg:       "create keys success",
		PublicKey: ret,
	}, nil
}

func (s *GRPCServer) SignTxMessage(_ context.Context, req *pb.SignTxMessageRequest) (*pb.SignTxMessageResponse, error) {
	privKey, ok := s.store.GetPrivKey(req.GetPublicKey())
	if !ok {
		return nil, fmt.Errorf("get private key by public key failed")
	}

	var (
		signature string
		err       error
	)

	switch req.GetType() {
	case "ecdsa":
		signature, err = crypto2.SignECDSAMessage(privKey, req.GetMessageHash())
	case "eddsa":
		signature, err = crypto2.SignEdDSAMessage(privKey, req.GetMessageHash())
	default:
		return nil, fmt.Errorf("unsupported key type")
	}
	if err != nil {
		return nil, err
	}

	return &pb.SignTxMessageResponse{Code: "1", Msg: "sign tx message success", Signature: signature}, nil
}
