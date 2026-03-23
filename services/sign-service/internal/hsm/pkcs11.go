package hsm

import "errors"

var ErrPKCS11ObjectNotFound = errors.New("pkcs11 object not found")
var ErrPKCS11ProviderUnavailable = errors.New("real pkcs11 provider requires linux+cgo with a compatible pkcs11 module")

type PKCS11Config struct {
	ClusterID  string
	Region     string
	User       string
	PIN        string
	ModulePath string
}

type PKCS11Provider interface {
	Open(cfg PKCS11Config) (PKCS11Session, error)
}

type PKCS11Session interface {
	Close() error
	LoadSeed(slotID string) ([]byte, error)
	StoreSeed(slotID string, seed []byte) error
}

type noopPKCS11Provider struct{}

func NewDefaultPKCS11Provider() PKCS11Provider {
	return newPlatformPKCS11Provider()
}

func NewNoopPKCS11Provider() PKCS11Provider {
	return &noopPKCS11Provider{}
}

func (p *noopPKCS11Provider) Open(cfg PKCS11Config) (PKCS11Session, error) {
	return nil, ErrCloudHSMNotImplemented
}
