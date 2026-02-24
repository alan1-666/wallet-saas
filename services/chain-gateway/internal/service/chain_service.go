package service

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"wallet-saas-v2/services/chain-gateway/internal/dispatcher"
	"wallet-saas-v2/services/chain-gateway/internal/ports"
)

type ChainService struct {
	Router *dispatcher.Router
}

type BuildUnsignedInput struct {
	Chain    string
	Network  string
	Base64Tx string
	Fee      string
	Vin      []ports.TxVin
	Vout     []ports.TxVout
}

type SendTxInput struct {
	Chain      string
	Network    string
	Coin       string
	RawTx      string
	UnsignedTx string
	Signatures []string
	PublicKeys []string
}

type SupportChainsInput struct {
	Chain   string
	Network string
}

type ValidAddressInput struct {
	Chain   string
	Network string
	Format  string
	Address string
}

func (s *ChainService) ConvertAddress(ctx context.Context, chain, addrType, publicKey string) (string, error) {
	adapter, err := s.Router.Resolve(chain)
	if err != nil {
		return "", err
	}
	return adapter.ConvertAddress(ctx, chain, addrType, publicKey)
}

func (s *ChainService) SupportChains(ctx context.Context, in SupportChainsInput) (bool, error) {
	adapter, err := s.Router.Resolve(in.Chain)
	if err != nil {
		return false, err
	}
	return adapter.SupportChains(ctx, in.Chain, in.Network)
}

func (s *ChainService) ValidAddress(ctx context.Context, in ValidAddressInput) (bool, error) {
	adapter, err := s.Router.Resolve(in.Chain)
	if err != nil {
		return false, err
	}
	return adapter.ValidAddress(ctx, in.Chain, in.Network, in.Format, in.Address)
}

func (s *ChainService) GetFee(ctx context.Context, in ports.FeeInput) (ports.FeeResult, error) {
	adapter, err := s.Router.Resolve(in.Chain)
	if err != nil {
		return ports.FeeResult{}, err
	}
	return adapter.GetFee(ctx, in)
}

func (s *ChainService) GetAccount(ctx context.Context, in ports.AccountInput) (ports.AccountResult, error) {
	adapter, err := s.Router.Resolve(in.Chain)
	if err != nil {
		return ports.AccountResult{}, err
	}
	return adapter.GetAccount(ctx, in)
}

func (s *ChainService) GetTxByHash(ctx context.Context, in ports.TxQueryInput) (json.RawMessage, error) {
	adapter, err := s.Router.Resolve(in.Chain)
	if err != nil {
		return nil, err
	}
	return adapter.GetTxByHash(ctx, in)
}

func (s *ChainService) GetTxByAddress(ctx context.Context, in ports.TxQueryInput) (json.RawMessage, error) {
	adapter, err := s.Router.Resolve(in.Chain)
	if err != nil {
		return nil, err
	}
	return adapter.GetTxByAddress(ctx, in)
}

func (s *ChainService) GetUnspentOutputs(ctx context.Context, chain, network, address string) (json.RawMessage, error) {
	adapter, err := s.Router.Resolve(chain)
	if err != nil {
		return nil, err
	}
	return adapter.GetUnspentOutputs(ctx, chain, network, address)
}

func (s *ChainService) BuildUnsigned(ctx context.Context, in BuildUnsignedInput) (ports.BuildUnsignedResult, error) {
	if in.Network == "" {
		in.Network = "mainnet"
	}
	adapter, err := s.Router.Resolve(in.Chain)
	if err != nil {
		return ports.BuildUnsignedResult{}, err
	}

	if len(in.Vin) > 0 || len(in.Vout) > 0 || in.Fee != "" {
		return adapter.BuildUnsignedUTXO(ctx, in.Chain, in.Network, in.Fee, in.Vin, in.Vout)
	}

	unsigned, err := adapter.BuildUnsignedAccount(ctx, in.Chain, in.Network, in.Base64Tx)
	if err != nil {
		if !errors.Is(err, ports.ErrUnsupported) {
			return ports.BuildUnsignedResult{}, err
		}
		return adapter.BuildUnsignedUTXO(ctx, in.Chain, in.Network, in.Fee, in.Vin, in.Vout)
	}
	return ports.BuildUnsignedResult{UnsignedTx: unsigned}, nil
}

func (s *ChainService) SendTx(ctx context.Context, in SendTxInput) (string, error) {
	if in.Network == "" {
		in.Network = "mainnet"
	}
	adapter, err := s.Router.Resolve(in.Chain)
	if err != nil {
		return "", err
	}

	rawTx := in.RawTx
	if rawTx == "" && in.UnsignedTx != "" && len(in.Signatures) > 0 && len(in.PublicKeys) > 0 {
		if len(in.Signatures) != len(in.PublicKeys) {
			return "", fmt.Errorf("signature count mismatch public_keys count")
		}
		txData, err := base64.StdEncoding.DecodeString(in.UnsignedTx)
		if err != nil {
			return "", fmt.Errorf("invalid unsigned_tx, require base64")
		}
		sigs := make([][]byte, 0, len(in.Signatures))
		for _, s := range in.Signatures {
			b, err := decodeHexOrBase64(s)
			if err != nil {
				return "", fmt.Errorf("invalid signature format")
			}
			sigs = append(sigs, b)
		}
		pubs := make([][]byte, 0, len(in.PublicKeys))
		for _, p := range in.PublicKeys {
			b, err := decodeHexOrBase64(p)
			if err != nil {
				return "", fmt.Errorf("invalid public key format")
			}
			pubs = append(pubs, b)
		}
		signedTxData, _, err := adapter.BuildSignedUTXO(ctx, in.Chain, in.Network, txData, sigs, pubs)
		if err != nil {
			return "", err
		}
		rawTx = string(signedTxData)
	}
	if rawTx == "" {
		return "", fmt.Errorf("raw_tx is required")
	}
	return adapter.SendTx(ctx, in.Chain, in.Network, in.Coin, rawTx)
}

func decodeHexOrBase64(s string) ([]byte, error) {
	if s == "" {
		return nil, errors.New("empty string")
	}
	if b, err := hex.DecodeString(s); err == nil {
		return b, nil
	}
	return base64.StdEncoding.DecodeString(s)
}
