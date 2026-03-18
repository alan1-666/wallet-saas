package orchestrator

import (
	"context"
	"strings"
	"testing"

	"wallet-saas-v2/services/wallet-core/internal/adapters/chain"
	"wallet-saas-v2/services/wallet-core/internal/ports"
)

func TestEnsureChainFundsSkipsNonSolanaTokens(t *testing.T) {
	o := &WithdrawOrchestrator{Chain: chain.NewMock()}
	err := o.ensureChainFunds(context.Background(), WithdrawRequest{
		Tx: ports.BuildUnsignedParams{
			Chain:  "ethereum",
			Coin:   "USDC",
			Amount: "1000",
		},
	})
	if err != nil {
		t.Fatalf("ensureChainFunds returned error: %v", err)
	}
}

func TestEnsureChainFundsRejectsWhenNativeBalanceInsufficient(t *testing.T) {
	mock := chain.NewMock()
	mock.Balances["FnnDB1kLNpkgAc5QgGnPF7N2YCh5oa7s9MbhT9eaM41M"] = "1000"
	mock.Balances["FnnDB1kLNpkgAc5QgGnPF7N2YCh5oa7s9MbhT9eaM41M|4zMMC9srt5Ri5X14GAgXhaHii3GnPAEERYPJgZJDncDU"] = "5000000"
	o := &WithdrawOrchestrator{Chain: mock}

	err := o.ensureChainFunds(context.Background(), WithdrawRequest{
		Tx: ports.BuildUnsignedParams{
			Chain:           "solana",
			Network:         "devnet",
			From:            "FnnDB1kLNpkgAc5QgGnPF7N2YCh5oa7s9MbhT9eaM41M",
			To:              "7nTj8m6P6xZgJ2EJgYQW1R6cM4zKq8GxSxXy5pX1b2YV",
			ContractAddress: "4zMMC9srt5Ri5X14GAgXhaHii3GnPAEERYPJgZJDncDU",
			Amount:          "1500000",
		},
	})
	if err == nil {
		t.Fatalf("expected insufficient balance error")
	}
	if !strings.Contains(err.Error(), "insufficient source SOL") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEnsureChainFundsAcceptsExistingATAAndSufficientBalance(t *testing.T) {
	mock := chain.NewMock()
	mock.Balances["FnnDB1kLNpkgAc5QgGnPF7N2YCh5oa7s9MbhT9eaM41M"] = "5000000"
	mock.Balances["FnnDB1kLNpkgAc5QgGnPF7N2YCh5oa7s9MbhT9eaM41M|4zMMC9srt5Ri5X14GAgXhaHii3GnPAEERYPJgZJDncDU"] = "5000000"
	ataAddress, err := solanaAssociatedTokenAddress("7nTj8m6P6xZgJ2EJgYQW1R6cM4zKq8GxSxXy5pX1b2YV", "4zMMC9srt5Ri5X14GAgXhaHii3GnPAEERYPJgZJDncDU")
	if err != nil {
		t.Fatalf("derive ata: %v", err)
	}
	mock.Balances[ataAddress] = "2039280"
	o := &WithdrawOrchestrator{Chain: mock}

	err = o.ensureChainFunds(context.Background(), WithdrawRequest{
		Tx: ports.BuildUnsignedParams{
			Chain:           "solana",
			Network:         "devnet",
			From:            "FnnDB1kLNpkgAc5QgGnPF7N2YCh5oa7s9MbhT9eaM41M",
			To:              "7nTj8m6P6xZgJ2EJgYQW1R6cM4zKq8GxSxXy5pX1b2YV",
			ContractAddress: "4zMMC9srt5Ri5X14GAgXhaHii3GnPAEERYPJgZJDncDU",
			Amount:          "1500000",
		},
	})
	if err != nil {
		t.Fatalf("ensureChainFunds returned error: %v", err)
	}
}

func TestEnsureChainFundsRejectsWhenTokenBalanceInsufficient(t *testing.T) {
	mock := chain.NewMock()
	mock.Balances["FnnDB1kLNpkgAc5QgGnPF7N2YCh5oa7s9MbhT9eaM41M"] = "5000000"
	mock.Balances["FnnDB1kLNpkgAc5QgGnPF7N2YCh5oa7s9MbhT9eaM41M|4zMMC9srt5Ri5X14GAgXhaHii3GnPAEERYPJgZJDncDU"] = "0"
	o := &WithdrawOrchestrator{Chain: mock}

	err := o.ensureChainFunds(context.Background(), WithdrawRequest{
		Tx: ports.BuildUnsignedParams{
			Chain:           "solana",
			Network:         "devnet",
			Coin:            "USDC",
			From:            "FnnDB1kLNpkgAc5QgGnPF7N2YCh5oa7s9MbhT9eaM41M",
			To:              "ATAedzhHX1Mf53X3CsUa53d69VZ12QSFf6nqN4AwZhcE",
			ContractAddress: "4zMMC9srt5Ri5X14GAgXhaHii3GnPAEERYPJgZJDncDU",
			Amount:          "1000000",
			AmountUnit:      "raw",
			TokenDecimals:   6,
		},
	})
	if err == nil {
		t.Fatalf("expected insufficient source token balance error")
	}
	if !strings.Contains(err.Error(), "insufficient source token balance on chain") {
		t.Fatalf("unexpected error: %v", err)
	}
}
