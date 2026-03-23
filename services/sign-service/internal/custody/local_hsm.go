package custody

import (
	"fmt"
	"strings"

	crypto2 "wallet-saas-v2/services/sign-service/internal/crypto"
	"wallet-saas-v2/services/sign-service/internal/hd"
	"wallet-saas-v2/services/sign-service/internal/hsm"
)

type LocalHSM struct {
	scheme     string
	slotPrefix string
	backend    hsm.Backend
}

func NewLocalHSM(backend hsm.Backend, slotPrefix, scheme string) (*LocalHSM, error) {
	if backend == nil {
		return nil, fmt.Errorf("hsm backend is required")
	}
	slotPrefix = strings.TrimSpace(slotPrefix)
	if slotPrefix == "" {
		slotPrefix = "master"
	}
	scheme = strings.TrimSpace(scheme)
	if scheme == "" {
		scheme = "local-hsm-slot"
	}
	return &LocalHSM{scheme: scheme, slotPrefix: slotPrefix, backend: backend}, nil
}

func (h *LocalHSM) Close() error {
	if h == nil || h.backend == nil {
		return nil
	}
	return h.backend.Close()
}

func (h *LocalHSM) CustodyScheme() string {
	if h == nil {
		return ""
	}
	return h.scheme
}

func (h *LocalHSM) DeriveKey(ref hd.KeyRef) (hd.DerivedKey, error) {
	seed, err := h.backend.LoadOrCreateSeed(h.slotID(ref.SignType))
	if err != nil {
		return hd.DerivedKey{}, err
	}
	return hd.DerivePublicKey(seed, ref)
}

func (h *LocalHSM) SignMessage(ref hd.KeyRef, messageHash string) (string, error) {
	seed, err := h.backend.LoadOrCreateSeed(h.slotID(ref.SignType))
	if err != nil {
		return "", err
	}
	signingKey, err := hd.DeriveSigningKey(seed, ref)
	if err != nil {
		return "", err
	}
	switch ref.SignType {
	case "ecdsa":
		return crypto2.SignECDSAMessage(signingKey.PrivateKeyHex, messageHash)
	case "eddsa":
		return crypto2.SignEdDSAMessage(signingKey.PrivateKeyHex, messageHash)
	default:
		return "", fmt.Errorf("unsupported sign type: %s", ref.SignType)
	}
}

func (h *LocalHSM) slotID(signType string) string {
	return strings.TrimSpace(h.slotPrefix) + ":" + strings.TrimSpace(signType)
}
