package custody

import "wallet-saas-v2/services/sign-service/internal/hd"

type Provider interface {
	Close() error
	CustodyScheme() string
	DeriveKey(ref hd.KeyRef) (hd.DerivedKey, error)
	SignMessage(ref hd.KeyRef, messageHash string) (string, error)
}
