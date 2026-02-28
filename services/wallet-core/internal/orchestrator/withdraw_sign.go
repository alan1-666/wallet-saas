package orchestrator

import (
	"context"
	"fmt"

	"wallet-saas-v2/services/wallet-core/internal/ports"
)

func (o *WithdrawOrchestrator) resolveKeys(req WithdrawRequest) ([]string, error) {
	keys := req.KeyIDs
	if len(keys) == 0 && req.KeyID != "" {
		keys = []string{req.KeyID}
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("missing key id")
	}
	return keys, nil
}

func resolveSignType(signType string, signHashes []string) string {
	if signType != "" {
		return signType
	}
	if len(signHashes) > 0 {
		return "ecdsa"
	}
	return "eddsa"
}

func (o *WithdrawOrchestrator) buildSignatures(ctx context.Context, signType string, keys []string, signHashes []string) ([]string, []string, error) {
	signatures := make([]string, 0, len(signHashes))
	publicKeys := make([]string, 0, len(signHashes))
	for i, signHash := range signHashes {
		keyIdx := i
		if keyIdx >= len(keys) {
			keyIdx = len(keys) - 1
		}
		sig, err := o.Sign.SignMessage(ctx, signType, keys[keyIdx], signHash)
		if err != nil {
			return nil, nil, err
		}
		signatures = append(signatures, sig)
		publicKeys = append(publicKeys, keys[keyIdx])
	}
	return signatures, publicKeys, nil
}

func (o *WithdrawOrchestrator) signAndBroadcastRaw(ctx context.Context, req WithdrawRequest, signType, keyID string, unsigned ports.BuildUnsignedResult) (string, error) {
	sig, err := o.Sign.SignMessage(ctx, signType, keyID, unsigned.UnsignedTx)
	if err != nil {
		return "", err
	}
	return o.broadcastRawTx(ctx, req, sig)
}
