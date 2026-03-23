//go:build !linux || !cgo

package hsm

type unavailablePKCS11Provider struct{}

func newPlatformPKCS11Provider() PKCS11Provider {
	return &unavailablePKCS11Provider{}
}

func (p *unavailablePKCS11Provider) Open(cfg PKCS11Config) (PKCS11Session, error) {
	return nil, ErrPKCS11ProviderUnavailable
}
