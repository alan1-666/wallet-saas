package hsm

import "wallet-saas-v2/services/sign-service/internal/keystore"

type SoftwareBackend struct {
	keys *keystore.Keys
}

func NewSoftwareBackend(path, namespace string) (*SoftwareBackend, error) {
	keys, err := keystore.New(path, namespace)
	if err != nil {
		return nil, err
	}
	return &SoftwareBackend{keys: keys}, nil
}

func (b *SoftwareBackend) Close() error {
	if b == nil || b.keys == nil {
		return nil
	}
	return b.keys.Close()
}

func (b *SoftwareBackend) LoadOrCreateSeed(slotID string) ([]byte, error) {
	return b.keys.LoadOrCreateSeed(slotID)
}
