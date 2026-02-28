package orchestrator

import (
	"context"

	"wallet-saas-v2/services/wallet-core/internal/ports"
)

func (o *WithdrawOrchestrator) broadcastRawTx(ctx context.Context, req WithdrawRequest, rawTx string) (string, error) {
	return o.Chain.Broadcast(ctx, ports.BroadcastParams{
		Chain:   req.Tx.Chain,
		Network: req.Tx.Network,
		Coin:    req.Tx.Coin,
		RawTx:   rawTx,
	})
}

func (o *WithdrawOrchestrator) broadcastUnsignedTx(ctx context.Context, req WithdrawRequest, unsignedTx string, signatures, publicKeys []string) (string, error) {
	return o.Chain.Broadcast(ctx, ports.BroadcastParams{
		Chain:      req.Tx.Chain,
		Network:    req.Tx.Network,
		Coin:       req.Tx.Coin,
		UnsignedTx: unsignedTx,
		Signatures: signatures,
		PublicKeys: publicKeys,
	})
}
