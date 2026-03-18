package chain

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"wallet-saas-v2/services/wallet-core/internal/ports"
)

func TestBuildBase64TxUsesLegacyFormatByDefault(t *testing.T) {
	got, err := buildBase64Tx(ports.BuildUnsignedParams{
		Chain:  "solana",
		From:   "from",
		To:     "to",
		Amount: "1000",
	})
	if err != nil {
		t.Fatalf("buildBase64Tx returned error: %v", err)
	}
	raw, err := base64.StdEncoding.DecodeString(got)
	if err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if string(raw) != "solana:from:to:1000" {
		t.Fatalf("unexpected legacy payload: %q", string(raw))
	}
}

func TestBuildBase64TxUsesStructuredSolanaPayloadForTokens(t *testing.T) {
	got, err := buildBase64Tx(ports.BuildUnsignedParams{
		Chain:           "solana",
		From:            "from",
		To:              "to",
		Amount:          "1500000",
		ContractAddress: "mint",
		AmountUnit:      "raw",
		TokenDecimals:   6,
	})
	if err != nil {
		t.Fatalf("buildBase64Tx returned error: %v", err)
	}
	raw, err := base64.StdEncoding.DecodeString(got)
	if err != nil {
		t.Fatalf("decode result: %v", err)
	}
	var payload accountTxPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.ContractAddress != "mint" || payload.Value != "1500000" || payload.AmountUnit != "raw" || payload.TokenDecimals != 6 {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}
