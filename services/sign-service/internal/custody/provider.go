package custody

import "wallet-saas-v2/services/sign-service/internal/hd"

type Provider interface {
	Close() error
	CustodyScheme() string
	DeriveKey(tenantID string, ref hd.KeyRef) (hd.DerivedKey, error)
	SignMessage(tenantID string, ref hd.KeyRef, messageHash string) (string, error)
}
