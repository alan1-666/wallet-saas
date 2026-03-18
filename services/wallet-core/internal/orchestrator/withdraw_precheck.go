package orchestrator

import (
	"context"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/gagliardetto/solana-go"

	"wallet-saas-v2/services/wallet-core/internal/ports"
)

const (
	solanaTokenFeeReserveLamports     uint64 = 1_000_000
	solanaAssociatedTokenRentLamports uint64 = 2_039_280
)

func (o *WithdrawOrchestrator) ensureChainFunds(ctx context.Context, req WithdrawRequest) error {
	if !isSolanaTokenWithdraw(req.Tx) {
		return nil
	}
	if strings.TrimSpace(req.Tx.From) == "" || strings.TrimSpace(req.Tx.To) == "" {
		return fmt.Errorf("solana token withdraw requires from/to address")
	}
	requiredTokenAmount, err := normalizeSolanaTokenAmount(req.Tx.Amount, req.Tx.AmountUnit, req.Tx.TokenDecimals)
	if err != nil {
		return err
	}
	tokenBalance, err := o.Chain.GetBalance(ctx, req.Tx.Chain, req.Tx.Coin, req.Tx.Network, req.Tx.From, req.Tx.ContractAddress)
	if err != nil {
		return fmt.Errorf("query source token balance: %w", err)
	}
	ok, err := gteAmount(tokenBalance.Balance, requiredTokenAmount)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("insufficient source token balance on chain: need at least %s, got %s", requiredTokenAmount, tokenBalance.Balance)
	}

	nativeBalance, err := o.Chain.GetBalance(ctx, req.Tx.Chain, "SOL", req.Tx.Network, req.Tx.From, "")
	if err != nil {
		return fmt.Errorf("query source native balance: %w", err)
	}

	requiredLamports := solanaTokenFeeReserveLamports
	ataAddress, err := solanaAssociatedTokenAddress(req.Tx.To, req.Tx.ContractAddress)
	if err != nil {
		requiredLamports += solanaAssociatedTokenRentLamports
	} else {
		ataBalance, balanceErr := o.Chain.GetBalance(ctx, req.Tx.Chain, "SOL", req.Tx.Network, ataAddress, "")
		if balanceErr != nil || isZeroBalance(ataBalance.Balance) {
			requiredLamports += solanaAssociatedTokenRentLamports
		}
	}

	requiredBalance := strconv.FormatUint(requiredLamports, 10)
	ok, err = gteAmount(nativeBalance.Balance, requiredBalance)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("insufficient source SOL for fee/rent: need at least %s lamports, got %s", requiredBalance, nativeBalance.Balance)
	}
	return nil
}

func isSolanaTokenWithdraw(tx ports.BuildUnsignedParams) bool {
	return strings.EqualFold(strings.TrimSpace(tx.Chain), "solana") && strings.TrimSpace(tx.ContractAddress) != ""
}

func solanaAssociatedTokenAddress(ownerAddress, mintAddress string) (string, error) {
	owner, err := solana.PublicKeyFromBase58(strings.TrimSpace(ownerAddress))
	if err != nil {
		return "", err
	}
	mint, err := solana.PublicKeyFromBase58(strings.TrimSpace(mintAddress))
	if err != nil {
		return "", err
	}
	ata, _, err := solana.FindAssociatedTokenAddress(owner, mint)
	if err != nil {
		return "", err
	}
	return ata.String(), nil
}

func isZeroBalance(v string) bool {
	n, ok := new(big.Int).SetString(strings.TrimSpace(v), 10)
	return !ok || n.Sign() <= 0
}

func gteAmount(actual, expected string) (bool, error) {
	actualInt, ok := new(big.Int).SetString(strings.TrimSpace(actual), 10)
	if !ok {
		return false, fmt.Errorf("invalid integer amount: %s", actual)
	}
	expectedInt, ok := new(big.Int).SetString(strings.TrimSpace(expected), 10)
	if !ok {
		return false, fmt.Errorf("invalid integer amount: %s", expected)
	}
	return actualInt.Cmp(expectedInt) >= 0, nil
}

func normalizeSolanaTokenAmount(value, amountUnit string, tokenDecimals uint32) (string, error) {
	normalizedValue := strings.TrimSpace(value)
	if normalizedValue == "" {
		return "", fmt.Errorf("token amount is required")
	}
	switch strings.ToLower(strings.TrimSpace(amountUnit)) {
	case "", "raw", "base", "base_unit", "base_units", "smallest":
		if strings.TrimSpace(amountUnit) == "" && strings.Contains(normalizedValue, ".") {
			if tokenDecimals == 0 {
				return "", fmt.Errorf("token_decimals is required when token amount uses display notation")
			}
			return normalizeDisplayAmount(normalizedValue, tokenDecimals)
		}
		if _, ok := new(big.Int).SetString(normalizedValue, 10); !ok {
			return "", fmt.Errorf("invalid integer amount: %s", normalizedValue)
		}
		return normalizedValue, nil
	case "display", "ui", "human":
		if tokenDecimals == 0 {
			return "", fmt.Errorf("token_decimals is required when amount_unit=display")
		}
		return normalizeDisplayAmount(normalizedValue, tokenDecimals)
	default:
		return "", fmt.Errorf("unsupported token amount unit: %s", amountUnit)
	}
}

func normalizeDisplayAmount(value string, tokenDecimals uint32) (string, error) {
	if strings.HasPrefix(value, "-") {
		return "", fmt.Errorf("token amount must be non-negative")
	}
	parts := strings.Split(value, ".")
	if len(parts) > 2 {
		return "", fmt.Errorf("invalid display token amount: %s", value)
	}
	intPart := parts[0]
	if intPart == "" {
		intPart = "0"
	}
	fracPart := ""
	if len(parts) == 2 {
		fracPart = parts[1]
	}
	if len(fracPart) > int(tokenDecimals) {
		return "", fmt.Errorf("display token amount exceeds supported precision: decimals=%d", tokenDecimals)
	}
	fracPart += strings.Repeat("0", int(tokenDecimals)-len(fracPart))

	intValue, ok := new(big.Int).SetString(intPart, 10)
	if !ok {
		return "", fmt.Errorf("invalid integer token amount: %s", value)
	}
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(tokenDecimals)), nil)
	total := new(big.Int).Mul(intValue, scale)
	if fracPart != "" {
		fracValue, ok := new(big.Int).SetString(fracPart, 10)
		if !ok {
			return "", fmt.Errorf("invalid fractional token amount: %s", value)
		}
		total.Add(total, fracValue)
	}
	return total.String(), nil
}
