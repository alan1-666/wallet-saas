package orchestrator

import "wallet-saas-v2/services/wallet-core/internal/ports"

type WithdrawOrchestrator struct {
	Ledger ports.LedgerPort
	Sign   ports.SignPort
	Chain  ports.ChainPort
}

type WithdrawRequest struct {
	TenantID      string
	AccountID     string
	OrderID       string
	RequiredConfs int64
	Signers       []ports.SignerRef
	SignType      string
	Tx            ports.BuildUnsignedParams
}

type BroadcastResult struct {
	TxHash     string
	UnsignedTx string
}
