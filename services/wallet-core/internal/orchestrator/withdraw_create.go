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

	unsignedResult, err := o.Chain.BuildUnsignedTx(ctx, req.Tx)
	if err != nil {
		return "", err
	}

	signers, err := o.resolveSigners(req)
	if err != nil {
		return "", err
	}
	signType := resolveSignType(req.SignType, req.Tx.Chain, unsignedResult.SignHashes)

	if len(unsignedResult.SignHashes) == 0 {
		txHash, err = o.signAndBroadcastRaw(ctx, req, signType, signers[0], unsignedResult)
		if err != nil {
			return "", err
		}
		broadcasted = true
		if err := o.confirmWithdraw(ctx, req, txHash); err != nil {
			return "", fmt.Errorf("withdraw broadcasted but ledger confirm failed, txHash=%s: %w", txHash, err)
		}
		return txHash, nil
	}

	signatures, publicKeys, err := o.buildSignatures(ctx, signType, req.Tx.Chain, signers, unsignedResult.SignHashes)
	if err != nil {
		return "", err
	}

	txHash, err = o.broadcastUnsignedTx(ctx, req, unsignedResult.UnsignedTx, signatures, publicKeys)
	if err != nil {
		return "", err
	}
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

	unsignedResult, err := o.Chain.BuildUnsignedTx(ctx, req.Tx)
	if err != nil {
		return "", err
	}

	signers, err := o.resolveSigners(req)
	if err != nil {
		return "", err
	}
	signType := resolveSignType(req.SignType, req.Tx.Chain, unsignedResult.SignHashes)

	if len(unsignedResult.SignHashes) == 0 {
		txHash, err = o.signAndBroadcastRaw(ctx, req, signType, signers[0], unsignedResult)
		if err != nil {
			return "", err
		}
	} else {
		signatures, publicKeys, err := o.buildSignatures(ctx, signType, req.Tx.Chain, signers, unsignedResult.SignHashes)
		if err != nil {
			return "", err
		}
		txHash, err = o.broadcastUnsignedTx(ctx, req, unsignedResult.UnsignedTx, signatures, publicKeys)
		if err != nil {
			return "", err
		}
	}

	if err := o.confirmWithdraw(ctx, req, txHash); err != nil {
		return "", fmt.Errorf("withdraw broadcasted but ledger confirm failed, txHash=%s: %w", txHash, err)
	}
	return txHash, nil
}
