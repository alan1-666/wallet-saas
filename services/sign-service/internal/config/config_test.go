package config

import "testing"

func TestValidateAllowsSoftwareBackend(t *testing.T) {
	cfg := Config{
		CustodyProvider:       "local-hsm",
		HSMBackend:            "software",
		SoftwareVaultPassword: "test-password",
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
}

func TestValidateRequiresCloudHSMFields(t *testing.T) {
	cfg := Config{
		CustodyProvider: "local-hsm",
		HSMBackend:      "cloudhsm",
	}
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected validation error")
	}
}
