package security

import "context"

type Scope struct {
	TenantID    string
	CanWithdraw bool
	CanDeposit  bool
	CanSweep    bool
}

type IdemResult struct {
	State    string
	Response string
}

type Provider interface {
	ValidateToken(ctx context.Context, token string) (Scope, error)
	CheckSignPermission(ctx context.Context, tenantID, keyID string) (bool, error)
	CheckTenantChainPolicy(ctx context.Context, tenantID, chain, network, operation, amount string) error
	Audit(ctx context.Context, tenantID, action, requestID, detail string) error

	Reserve(ctx context.Context, tenantID, requestID, operation, requestHash string) (IdemResult, error)
	Commit(ctx context.Context, tenantID, requestID, operation, response string) error
	Reject(ctx context.Context, tenantID, requestID, operation, reason string) error
	Close() error
}
