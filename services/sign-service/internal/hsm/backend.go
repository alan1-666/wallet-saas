package hsm

type Backend interface {
	Close() error
	LoadOrCreateSeed(slotID string) ([]byte, error)
}
