package worker

import (
	"fmt"
	"math/big"
	"strings"
)

func depositOrderID(txHash string, eventIndex int64, accountID, network string) string {
	return fmt.Sprintf("dep_%s_%d_%s_%s", txHash, eventIndex, accountID, normalizePart(network))
}

func sweepOrderID(txHash string, eventIndex int64, accountID, network string) string {
	return fmt.Sprintf("sweep_%s_%d_%s_%s", txHash, eventIndex, accountID, normalizePart(network))
}

func depositEventKey(tenantID, chain, network, txHash string, eventIndex int64) string {
	return fmt.Sprintf("dep:%s:%s:%s:%s:%d", normalizePart(tenantID), normalizePart(chain), normalizePart(network), strings.ToLower(strings.TrimSpace(txHash)), eventIndex)
}

func sweepEventKey(tenantID, chain, network, txHash string, eventIndex int64) string {
	return fmt.Sprintf("sweep:%s:%s:%s:%s:%d", normalizePart(tenantID), normalizePart(chain), normalizePart(network), strings.ToLower(strings.TrimSpace(txHash)), eventIndex)
}

func fallback(v, d string) string {
	if strings.TrimSpace(v) == "" {
		return d
	}
	return v
}

func normalizePart(v string) string {
	x := strings.ToLower(strings.TrimSpace(v))
	if x == "" {
		return "unknown"
	}
	return strings.ReplaceAll(x, " ", "_")
}

func meetsThreshold(amount, threshold string) bool {
	if strings.TrimSpace(threshold) == "" {
		return true
	}
	a, ok := new(big.Int).SetString(strings.TrimSpace(amount), 10)
	if !ok {
		return false
	}
	t, ok := new(big.Int).SetString(strings.TrimSpace(threshold), 10)
	if !ok {
		return false
	}
	return a.Cmp(t) > 0
}

func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func resolveDepositStatus(rawStatus string, confirmations, minConf int64) string {
	status := strings.ToUpper(strings.TrimSpace(rawStatus))
	if status == "REVERTED" || status == "FAILED" {
		return "REVERTED"
	}
	if status == "PENDING" {
		return "PENDING"
	}
	if confirmations < 0 {
		if status == "CONFIRMED" {
			return "CONFIRMED"
		}
		return "PENDING"
	}
	if minConf <= 0 {
		minConf = 1
	}
	if confirmations >= 0 && confirmations < minConf {
		return "PENDING"
	}
	return "CONFIRMED"
}
