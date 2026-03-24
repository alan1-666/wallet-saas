package hsm

type Backend interface {
	Close() error
	LoadOrCreateSeed(slotID string) ([]byte, error)
}

type SeedProvisioner interface {
	ProvisionSeed(slotID string, seed []byte) error
}

type SeedManager interface {
	SeedProvisioner
	ExportSeed(slotID string) ([]byte, error)
	ReplaceSeed(slotID string, seed []byte) error
}
