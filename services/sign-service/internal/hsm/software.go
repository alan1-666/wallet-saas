package hsm

import "wallet-saas-v2/services/sign-service/internal/keystore"

type SoftwareBackend struct {
	keys *keystore.Keys
}

type SoftwareConfig struct {
	Path       string
	Namespace  string
	Password   string
	AutoCreate bool
}

func NewSoftwareBackend(cfg SoftwareConfig) (*SoftwareBackend, error) {
	keys, err := keystore.New(cfg.Path, cfg.Namespace, cfg.Password, cfg.AutoCreate)
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

func (b *SoftwareBackend) ProvisionSeed(slotID string, seed []byte) error {
	return b.keys.ProvisionSeed(slotID, seed)
}

func (b *SoftwareBackend) ExportSeed(slotID string) ([]byte, error) {
	return b.keys.LoadSeed(slotID)
}

func (b *SoftwareBackend) ReplaceSeed(slotID string, seed []byte) error {
	return b.keys.ReplaceSeed(slotID, seed)
}
