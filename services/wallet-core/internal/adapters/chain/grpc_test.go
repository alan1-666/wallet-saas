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

func TestBuildBase64TxUsesStructuredTronPayload(t *testing.T) {
	got, err := buildBase64Tx(ports.BuildUnsignedParams{
		Chain:   "tron",
		From:    "TFromAddress",
		To:      "TToAddress",
		Amount:  "1000",
		Network: "nile",
	})
	if err != nil {
		t.Fatalf("buildBase64Tx returned error: %v", err)
	}
	raw, err := base64.StdEncoding.DecodeString(got)
	if err != nil {
		t.Fatalf("decode result: %v", err)
	}
	var payload tronAccountTxPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.FromAddress != "TFromAddress" || payload.ToAddress != "TToAddress" || payload.Value != 1000 {
		t.Fatalf("unexpected tron payload: %+v", payload)
	}
}

func TestBuildBase64TxUsesStructuredEVMPayloadForSupportedChains(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		chain   string
		network string
		chainID string
	}{
		{name: "ethereum sepolia", chain: "ethereum", network: "sepolia", chainID: "11155111"},
		{name: "binance testnet", chain: "binance", network: "testnet", chainID: "97"},
		{name: "polygon amoy", chain: "polygon", network: "amoy", chainID: "80002"},
		{name: "arbitrum sepolia", chain: "arbitrum", network: "sepolia", chainID: "421614"},
		{name: "optimism sepolia", chain: "optimism", network: "sepolia", chainID: "11155420"},
		{name: "linea sepolia", chain: "linea", network: "sepolia", chainID: "59141"},
		{name: "scroll sepolia", chain: "scroll", network: "sepolia", chainID: "534351"},
		{name: "mantle sepolia", chain: "mantle", network: "sepolia", chainID: "5003"},
		{name: "zksync sepolia", chain: "zksync", network: "sepolia", chainID: "300"},
		{name: "optimism mainnet", chain: "optimism", network: "mainnet", chainID: "10"},
		{name: "linea mainnet", chain: "linea", network: "mainnet", chainID: "59144"},
		{name: "scroll mainnet", chain: "scroll", network: "mainnet", chainID: "534352"},
		{name: "mantle mainnet", chain: "mantle", network: "mainnet", chainID: "5000"},
		{name: "zksync mainnet", chain: "zksync", network: "mainnet", chainID: "324"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := buildBase64Tx(ports.BuildUnsignedParams{
				Chain:   tc.chain,
				Network: tc.network,
				From:    "0x1111111111111111111111111111111111111111",
				To:      "0x2222222222222222222222222222222222222222",
				Amount:  "12345",
			})
			if err != nil {
				t.Fatalf("buildBase64Tx returned error: %v", err)
			}

			raw, err := base64.StdEncoding.DecodeString(got)
			if err != nil {
				t.Fatalf("decode result: %v", err)
			}

			var payload evmDynamicFeePayload
			if err := json.Unmarshal(raw, &payload); err != nil {
				t.Fatalf("unmarshal payload: %v", err)
			}

			if payload.ChainID != tc.chainID {
				t.Fatalf("unexpected chainId: got=%s want=%s payload=%+v", payload.ChainID, tc.chainID, payload)
			}
			if payload.NonceMode != "pending" {
				t.Fatalf("expected pending nonce mode, got payload=%+v", payload)
			}
			if payload.FromAddress != "0x1111111111111111111111111111111111111111" ||
				payload.ToAddress != "0x2222222222222222222222222222222222222222" ||
				payload.Amount != "12345" {
				t.Fatalf("unexpected payload: %+v", payload)
			}
			if payload.GasLimit != 21000 {
				t.Fatalf("unexpected gas limit: %+v", payload)
			}
		})
	}
}

func TestBuildBase64TxUsesHigherGasLimitForEVMTokenTransfer(t *testing.T) {
	got, err := buildBase64Tx(ports.BuildUnsignedParams{
		Chain:           "optimism",
		Network:         "sepolia",
		From:            "0x1111111111111111111111111111111111111111",
		To:              "0x2222222222222222222222222222222222222222",
		Amount:          "12345",
		ContractAddress: "0x3333333333333333333333333333333333333333",
	})
	if err != nil {
		t.Fatalf("buildBase64Tx returned error: %v", err)
	}

	raw, err := base64.StdEncoding.DecodeString(got)
	if err != nil {
		t.Fatalf("decode result: %v", err)
	}

	var payload evmDynamicFeePayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.ChainID != "11155420" {
		t.Fatalf("unexpected chainId: %+v", payload)
	}
	if payload.ContractAddress != "0x3333333333333333333333333333333333333333" || payload.GasLimit != 100000 {
		t.Fatalf("unexpected token payload: %+v", payload)
	}
}
