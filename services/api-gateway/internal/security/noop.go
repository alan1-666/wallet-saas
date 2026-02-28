package security

import (
	"context"
	"errors"
)

type Noop struct{}

func NewNoop() *Noop {
	return &Noop{}
}

func (n *Noop) ValidateToken(context.Context, string) (Scope, error) {
	return Scope{}, errors.New("auth provider not configured")
}

func (n *Noop) CheckSignPermission(context.Context, string, string) (bool, error) {
	return false, errors.New("auth provider not configured")
}

func (n *Noop) CheckTenantChainPolicy(context.Context, string, string, string, string, string) error {
	return errors.New("policy provider not configured")
}

func (n *Noop) Audit(context.Context, string, string, string, string) error {
	return nil
}

func (n *Noop) Reserve(context.Context, string, string, string, string) (IdemResult, error) {
	return IdemResult{}, errors.New("idempotency provider not configured")
}

func (n *Noop) Commit(context.Context, string, string, string, string) error {
	return nil
}

func (n *Noop) Reject(context.Context, string, string, string, string) error {
	return nil
}

func (n *Noop) Close() error { return nil }
