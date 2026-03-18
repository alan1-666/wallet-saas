package orchestrator

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/mr-tron/base58"

	"wallet-saas-v2/services/wallet-core/internal/ports"
)

func (o *WithdrawOrchestrator) resolveSigners(req WithdrawRequest) ([]ports.SignerRef, error) {
	if len(req.Signers) == 0 {
		return nil, fmt.Errorf("missing key id")
	}
	return req.Signers, nil
}

func resolveSignType(signType, chain string, signHashes []string) string {
	if signType != "" {
		return signType
	}
	if chainUsesEdDSA(chain) {
		return "eddsa"
	}
	if len(signHashes) > 0 {
		return "ecdsa"
	}
	return "eddsa"
}

func chainUsesEdDSA(chain string) bool {
	switch strings.ToLower(strings.TrimSpace(chain)) {
	case "sol", "solana", "apt", "aptos", "sui", "ton", "xlm", "stellar":
		return true
	default:
		return false
	}
}

func (o *WithdrawOrchestrator) buildSignatures(ctx context.Context, signType, chain string, signers []ports.SignerRef, signHashes []string) ([]string, []string, error) {
	signatures := make([]string, 0, len(signHashes))
	publicKeys := make([]string, 0, len(signHashes))
	for i, signHash := range signHashes {
		keyIdx := i
		if keyIdx >= len(signers) {
			keyIdx = len(signers) - 1
		}
		signer := signers[keyIdx]
		sig, err := o.Sign.SignMessage(ctx, signType, signer.KeyID, signHash)
		if err != nil {
			return nil, nil, err
		}
		signatures = append(signatures, sig)
		publicKeys = append(publicKeys, normalizeBroadcastPublicKey(chain, signer.PublicKey))
	}
	return signatures, publicKeys, nil
}

func (o *WithdrawOrchestrator) signAndBroadcastRaw(ctx context.Context, req WithdrawRequest, signType string, signer ports.SignerRef, unsigned ports.BuildUnsignedResult) (string, error) {
	sig, err := o.Sign.SignMessage(ctx, signType, signer.KeyID, unsigned.UnsignedTx)
	if err != nil {
		return "", err
	}
	return o.broadcastRawTx(ctx, req, sig)
}

func normalizeBroadcastPublicKey(chain, key string) string {
	if !isSolanaChain(chain) {
		return key
	}
	k := strings.TrimSpace(key)
	h := strings.TrimPrefix(strings.ToLower(k), "0x")
	if len(h) != 64 {
		return key
	}
	raw, err := hex.DecodeString(h)
	if err != nil || len(raw) != 32 {
		return key
	}
	if out := base58.Encode(raw); strings.TrimSpace(out) != "" {
		return out
	}
	return key
}

func isSolanaChain(chain string) bool {
	switch strings.ToLower(strings.TrimSpace(chain)) {
	case "sol", "solana":
		return true
	default:
		return false
	}
}
