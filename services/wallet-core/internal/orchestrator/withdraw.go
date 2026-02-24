package orchestrator

import (
	"context"
	"fmt"

	"wallet-saas-v2/services/wallet-core/internal/ports"
)

type WithdrawOrchestrator struct {
	Risk   ports.RiskPort
	Ledger ports.LedgerPort
	Sign   ports.SignPort
	Chain  ports.ChainPort
}

type WithdrawRequest struct {
	TenantID  string
	AccountID string
	OrderID   string
	KeyID     string
	KeyIDs    []string
	SignType  string
	Tx        ports.BuildUnsignedParams
}

func (o *WithdrawOrchestrator) CreateAndBroadcast(ctx context.Context, req WithdrawRequest) (txHash string, err error) {
	decision, err := o.Risk.CheckWithdraw(ctx, req.TenantID, req.AccountID, req.OrderID, req.Tx.Chain, req.Tx.Coin, req.Tx.Amount)
	if err != nil {
		return "", err
	}
	if decision != "ALLOW" {
		return "", fmt.Errorf("withdraw blocked by risk: %s", decision)
	}

	if err := o.Ledger.FreezeWithdraw(ctx, req.TenantID, req.AccountID, req.OrderID, req.Tx.Coin, req.Tx.Amount); err != nil {
		return "", err
	}
	frozen := true
	broadcasted := false
	defer func() {
		if err == nil || !frozen || broadcasted {
			return
		}
		reason := err.Error()
		if releaseErr := o.Ledger.ReleaseWithdraw(ctx, req.TenantID, req.AccountID, req.OrderID, reason); releaseErr != nil {
			err = fmt.Errorf("%v; release failed: %v", err, releaseErr)
		}
	}()

	unsignedResult, err := o.Chain.BuildUnsignedTx(ctx, req.Tx)
	if err != nil {
		return "", err
	}

	keys := req.KeyIDs
	if len(keys) == 0 && req.KeyID != "" {
		keys = []string{req.KeyID}
	}
	if len(keys) == 0 {
		return "", fmt.Errorf("missing key id")
	}

	signType := req.SignType
	if signType == "" {
		signType = "eddsa"
		if len(unsignedResult.SignHashes) > 0 {
			signType = "ecdsa"
		}
	}

	if len(unsignedResult.SignHashes) == 0 {
		sig, err := o.Sign.SignMessage(ctx, signType, keys[0], unsignedResult.UnsignedTx)
		if err != nil {
			return "", err
		}
		txHash, err = o.Chain.Broadcast(ctx, ports.BroadcastParams{
			Chain:   req.Tx.Chain,
			Network: req.Tx.Network,
			Coin:    req.Tx.Coin,
			RawTx:   sig,
		})
		if err != nil {
			return "", err
		}
		broadcasted = true
		if err := o.Ledger.ConfirmWithdraw(ctx, req.TenantID, req.AccountID, req.OrderID, txHash); err != nil {
			return "", fmt.Errorf("withdraw broadcasted but ledger confirm failed, txHash=%s: %w", txHash, err)
		}
		return txHash, nil
	}

	signatures := make([]string, 0, len(unsignedResult.SignHashes))
	publicKeys := make([]string, 0, len(unsignedResult.SignHashes))
	for i, signHash := range unsignedResult.SignHashes {
		keyIdx := i
		if keyIdx >= len(keys) {
			keyIdx = len(keys) - 1
		}
		sig, err := o.Sign.SignMessage(ctx, signType, keys[keyIdx], signHash)
		if err != nil {
			return "", err
		}
		signatures = append(signatures, sig)
		publicKeys = append(publicKeys, keys[keyIdx])
	}

	txHash, err = o.Chain.Broadcast(ctx, ports.BroadcastParams{
		Chain:      req.Tx.Chain,
		Network:    req.Tx.Network,
		Coin:       req.Tx.Coin,
		UnsignedTx: unsignedResult.UnsignedTx,
		Signatures: signatures,
		PublicKeys: publicKeys,
	})
	if err != nil {
		return "", err
	}
	broadcasted = true
	if err := o.Ledger.ConfirmWithdraw(ctx, req.TenantID, req.AccountID, req.OrderID, txHash); err != nil {
		return "", fmt.Errorf("withdraw broadcasted but ledger confirm failed, txHash=%s: %w", txHash, err)
	}
	return txHash, nil
}
