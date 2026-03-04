package service

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"

	"wallet-saas-v2/services/chain-gateway/internal/ports"
)

func (s *ChainService) BuildUnsigned(ctx context.Context, in BuildUnsignedInput) (ports.BuildUnsignedResult, error) {
	if err := validateChainNetwork(in.Chain, in.Network); err != nil {
		return ports.BuildUnsignedResult{}, err
	}
	binding, err := s.Router.Resolve(in.Chain, in.Network)
	if err != nil {
		return ports.BuildUnsignedResult{}, err
	}

	if len(in.Vin) > 0 || len(in.Vout) > 0 || in.Fee != "" {
		return withRetry(ctx, "build-unsigned-utxo", func() (ports.BuildUnsignedResult, error) {
			return binding.Adapter.BuildUnsignedUTXO(ctx, binding.Chain, in.Network, in.Fee, in.Vin, in.Vout)
		})
	}

	unsigned, err := withRetry(ctx, "build-unsigned-account", func() (ports.BuildUnsignedResult, error) {
		return binding.Adapter.BuildUnsignedAccount(ctx, binding.Chain, in.Network, in.Base64Tx)
	})
	if err != nil {
		if !errors.Is(err, ports.ErrUnsupported) {
			return ports.BuildUnsignedResult{}, err
		}
		return withRetry(ctx, "build-unsigned-utxo-fallback", func() (ports.BuildUnsignedResult, error) {
			return binding.Adapter.BuildUnsignedUTXO(ctx, binding.Chain, in.Network, in.Fee, in.Vin, in.Vout)
		})
	}
	return unsigned, nil
}

func (s *ChainService) SendTx(ctx context.Context, in SendTxInput) (string, error) {
	if err := validateChainNetwork(in.Chain, in.Network); err != nil {
		return "", err
	}
	binding, err := s.Router.Resolve(in.Chain, in.Network)
	if err != nil {
		return "", err
	}

	rawTx := in.RawTx
	if rawTx == "" && in.UnsignedTx != "" && len(in.Signatures) > 0 && len(in.PublicKeys) > 0 {
		if len(in.Signatures) != len(in.PublicKeys) {
			return "", fmt.Errorf("signature count mismatch public_keys count")
		}
		if binding.Model == ports.ModelAccount {
			if len(in.Signatures) != 1 || len(in.PublicKeys) != 1 {
				return "", fmt.Errorf("account model supports single signature, got signatures=%d public_keys=%d", len(in.Signatures), len(in.PublicKeys))
			}
			signedTx, err := s.buildSignedAccount(ctx, binding, in.Network, in.UnsignedTx, in.Signatures[0], in.PublicKeys[0])
			if err != nil {
				return "", err
			}
			rawTx = signedTx
		} else {
			txData, err := base64.StdEncoding.DecodeString(in.UnsignedTx)
			if err != nil {
				return "", fmt.Errorf("invalid unsigned_tx, require base64")
			}
			sigs := make([][]byte, 0, len(in.Signatures))
			for _, sig := range in.Signatures {
				b, err := decodeHexOrBase64(sig)
				if err != nil {
					return "", fmt.Errorf("invalid signature format")
				}
				sigs = append(sigs, b)
			}
			pubs := make([][]byte, 0, len(in.PublicKeys))
			for _, pub := range in.PublicKeys {
				b, err := decodeHexOrBase64(pub)
				if err != nil {
					return "", fmt.Errorf("invalid public key format")
				}
				pubs = append(pubs, b)
			}
			signedTxData, err := s.buildSignedUTXO(ctx, binding, in.Network, txData, sigs, pubs)
			if err != nil {
				return "", err
			}
			rawTx = string(signedTxData)
		}
	}
	if rawTx == "" {
		return "", fmt.Errorf("raw_tx is required")
	}
	return withRetry(ctx, "send-tx", func() (string, error) {
		return binding.Adapter.SendTx(ctx, binding.Chain, in.Network, in.Coin, rawTx)
	})
}

func (s *ChainService) buildSignedUTXO(ctx context.Context, binding ports.PluginBinding, network string, txData []byte, signatures [][]byte, publicKeys [][]byte) ([]byte, error) {
	var (
		out []byte
		err error
	)
	_, err = withRetry(ctx, "build-signed-utxo", func() (struct{}, error) {
		out, _, err = binding.Adapter.BuildSignedUTXO(ctx, binding.Chain, network, txData, signatures, publicKeys)
		if err != nil {
			return struct{}{}, err
		}
		return struct{}{}, nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (s *ChainService) buildSignedAccount(ctx context.Context, binding ports.PluginBinding, network, base64Tx, signature, publicKey string) (string, error) {
	return withRetry(ctx, "build-signed-account", func() (string, error) {
		return binding.Adapter.BuildSignedAccount(ctx, binding.Chain, network, base64Tx, signature, publicKey)
	})
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
