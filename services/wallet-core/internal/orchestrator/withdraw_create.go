package orchestrator

import (
	"context"
	"fmt"
)

func (o *WithdrawOrchestrator) CreateAndBroadcast(ctx context.Context, req WithdrawRequest) (txHash string, err error) {
	if err := o.ensureChainFunds(ctx, req); err != nil {
		return "", err
	}

	if err := o.freezeWithdraw(ctx, req); err != nil {
		return "", err
	}
	frozen := true
	broadcasted := false
	defer func() {
		if err == nil || !frozen || broadcasted {
			return
		}
		reason := err.Error()
		if releaseErr := o.releaseWithdraw(ctx, req, reason); releaseErr != nil {
			err = fmt.Errorf("%v; release failed: %v", err, releaseErr)
		}
	}()

	broadcast, err := o.buildSignAndBroadcast(ctx, req)
	if err != nil {
		return "", err
	}
	txHash = broadcast.TxHash
	broadcasted = true

	if err := o.confirmWithdraw(ctx, req, txHash); err != nil {
		return "", fmt.Errorf("withdraw broadcasted but ledger confirm failed, txHash=%s: %w", txHash, err)
	}
	return txHash, nil
}

func (o *WithdrawOrchestrator) ExecuteQueued(ctx context.Context, req WithdrawRequest) (txHash string, err error) {
	if err := o.ensureChainFunds(ctx, req); err != nil {
		return "", err
	}

	broadcast, err := o.buildSignAndBroadcast(ctx, req)
	if err != nil {
		return "", err
	}
	txHash = broadcast.TxHash

	if err := o.confirmWithdraw(ctx, req, txHash); err != nil {
		return "", fmt.Errorf("withdraw broadcasted but ledger confirm failed, txHash=%s: %w", txHash, err)
	}
	return txHash, nil
}

func (o *WithdrawOrchestrator) ExecuteQueuedWithUnsigned(ctx context.Context, req WithdrawRequest) (BroadcastResult, error) {
	if err := o.ensureChainFunds(ctx, req); err != nil {
		return BroadcastResult{}, err
	}
	broadcast, err := o.buildSignAndBroadcast(ctx, req)
	if err != nil {
		return BroadcastResult{}, err
	}
	if err := o.confirmWithdraw(ctx, req, broadcast.TxHash); err != nil {
		return BroadcastResult{}, fmt.Errorf("withdraw broadcasted but ledger confirm failed, txHash=%s: %w", broadcast.TxHash, err)
	}
	return broadcast, nil
}

func (o *WithdrawOrchestrator) SpeedUpBroadcasted(ctx context.Context, req WithdrawRequest) (BroadcastResult, error) {
	if err := o.ensureChainFunds(ctx, req); err != nil {
		return BroadcastResult{}, err
	}
	return o.buildSignAndBroadcast(ctx, req)
}

func (o *WithdrawOrchestrator) BroadcastOnly(ctx context.Context, req WithdrawRequest) (BroadcastResult, error) {
	if err := o.ensureChainFunds(ctx, req); err != nil {
		return BroadcastResult{}, err
	}
	return o.buildSignAndBroadcast(ctx, req)
}

func (o *WithdrawOrchestrator) buildSignAndBroadcast(ctx context.Context, req WithdrawRequest) (BroadcastResult, error) {
	unsignedResult, err := o.Chain.BuildUnsignedTx(ctx, req.Tx)
	if err != nil {
		return BroadcastResult{}, err
	}

	signers, err := o.resolveSigners(req)
	if err != nil {
		return BroadcastResult{}, err
	}
	signType := resolveSignType(req.SignType, req.Tx.Chain, unsignedResult.SignHashes)

	if len(unsignedResult.SignHashes) == 0 {
		txHash, err := o.signAndBroadcastRaw(ctx, req, signType, signers[0], unsignedResult)
		if err != nil {
			return BroadcastResult{}, err
		}
		return BroadcastResult{TxHash: txHash, UnsignedTx: unsignedResult.UnsignedTx}, nil
	}

	signatures, publicKeys, err := o.buildSignatures(ctx, req.TenantID, signType, req.Tx.Chain, signers, unsignedResult.SignHashes)
	if err != nil {
		return BroadcastResult{}, err
	}
	txHash, err := o.broadcastUnsignedTx(ctx, req, unsignedResult.UnsignedTx, signatures, publicKeys)
	if err != nil {
		return BroadcastResult{}, err
	}
	return BroadcastResult{TxHash: txHash, UnsignedTx: unsignedResult.UnsignedTx}, nil
}
