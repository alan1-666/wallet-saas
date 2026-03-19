package worker

import (
	"fmt"
	"math/big"
	"strings"
	"time"
)

const (
	depositScanStatusSeen      = "SEEN"
	depositScanStatusPending   = "PENDING"
	depositScanStatusConfirmed = "CONFIRMED"
	depositScanStatusFinalized = "FINALIZED"
	depositScanStatusReorged   = "REORGED"
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

func projectDepositEventKey(tenantID, chain, network, txHash string, eventIndex int64) string {
	return fmt.Sprintf("project-dep:%s:%s:%s:%s:%d", normalizePart(tenantID), normalizePart(chain), normalizePart(network), strings.ToLower(strings.TrimSpace(txHash)), eventIndex)
}

func sweepTriggerEventKey(tenantID, chain, network, txHash string, eventIndex int64) string {
	return fmt.Sprintf("sweep-trigger:%s:%s:%s:%s:%d", normalizePart(tenantID), normalizePart(chain), normalizePart(network), strings.ToLower(strings.TrimSpace(txHash)), eventIndex)
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

func resolveDepositScanStatus(rawStatus string, confirmations, minConf, reorgWindow int64) string {
	status := strings.ToUpper(strings.TrimSpace(rawStatus))
	switch status {
	case depositScanStatusReorged, "REVERTED", "FAILED":
		return depositScanStatusReorged
	case depositScanStatusFinalized:
		return depositScanStatusFinalized
	}
	if minConf <= 0 {
		minConf = 1
	}
	if reorgWindow < 0 {
		reorgWindow = 0
	}
	if confirmations < 0 {
		return depositScanStatusSeen
	}
	if confirmations == 0 {
		return depositScanStatusSeen
	}
	if confirmations < minConf {
		return depositScanStatusPending
	}
	if reorgWindow > 0 && confirmations >= minConf+reorgWindow {
		return depositScanStatusFinalized
	}
	return depositScanStatusConfirmed
}

func mapDepositLedgerStatus(scanStatus string) string {
	switch strings.ToUpper(strings.TrimSpace(scanStatus)) {
	case depositScanStatusConfirmed, depositScanStatusFinalized:
		return "CONFIRMED"
	case depositScanStatusReorged, "REVERTED", "FAILED":
		return "REVERTED"
	default:
		return "PENDING"
	}
}

func shouldProjectNotify(oldScanStatus, newScanStatus string) bool {
	return mapDepositLedgerStatus(oldScanStatus) != "CONFIRMED" && mapDepositLedgerStatus(newScanStatus) == "CONFIRMED"
}

func shouldTriggerSweep(autoSweep bool, accountID, treasuryID, oldScanStatus, newScanStatus string) bool {
	if !autoSweep {
		return false
	}
	if strings.TrimSpace(accountID) != "" && strings.TrimSpace(accountID) == strings.TrimSpace(treasuryID) {
		return false
	}
	return shouldProjectNotify(oldScanStatus, newScanStatus)
}

type paginationGuard struct {
	maxEmptyPages    int
	cursorStallGuard int
	emptyPages       int
	seenCursors      map[string]int
}

func newPaginationGuard(maxEmptyPages, cursorStallGuard int) *paginationGuard {
	if maxEmptyPages < 0 {
		maxEmptyPages = 0
	}
	if cursorStallGuard < 0 {
		cursorStallGuard = 0
	}
	return &paginationGuard{
		maxEmptyPages:    maxEmptyPages,
		cursorStallGuard: cursorStallGuard,
		seenCursors:      make(map[string]int),
	}
}

func (g *paginationGuard) Observe(currentCursor, nextCursor string, itemCount int) (advance bool, stop bool, reason string) {
	if g == nil {
		return nextCursor != "" && nextCursor != currentCursor, nextCursor == "" || nextCursor == currentCursor, ""
	}
	if itemCount == 0 {
		g.emptyPages++
	} else {
		g.emptyPages = 0
	}
	if strings.TrimSpace(nextCursor) == "" {
		return false, true, "cursor_exhausted"
	}
	if nextCursor == currentCursor {
		return false, true, "cursor_not_advanced"
	}
	if g.maxEmptyPages > 0 && itemCount == 0 && g.emptyPages >= g.maxEmptyPages {
		return false, true, "max_empty_pages"
	}
	if g.cursorStallGuard > 0 {
		g.seenCursors[nextCursor]++
		if g.seenCursors[nextCursor] > g.cursorStallGuard {
			return false, true, "cursor_stall_guard"
		}
	}
	return true, false, ""
}

func isSolanaChain(chain string) bool {
	switch strings.ToLower(strings.TrimSpace(chain)) {
	case "sol", "solana":
		return true
	default:
		return false
	}
}

func shouldSkipInternalTransfer(fromAddr string, managedAddrs map[string]struct{}) bool {
	if len(managedAddrs) == 0 {
		return false
	}
	key := strings.ToLower(strings.TrimSpace(fromAddr))
	if key == "" {
		return false
	}
	_, ok := managedAddrs[key]
	return ok
}

func shouldFailOutgoingNotFound(chain string, age time.Duration, missCount, threshold int64, grace time.Duration) bool {
	if !isSolanaChain(chain) {
		return false
	}
	if threshold <= 0 {
		threshold = 1
	}
	if grace < 0 {
		grace = 0
	}
	if missCount < threshold {
		return false
	}
	return age >= grace
}
