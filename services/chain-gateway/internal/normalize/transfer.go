package normalize

import (
	"encoding/json"
	"strconv"
	"strings"

	"wallet-saas-v2/services/chain-gateway/internal/ports"
)

func IncomingTransfers(model ports.ChainModel, chain string, raw map[string]any, watchAddress string) ports.IncomingTransferResult {
	if model == ports.ModelUTXO {
		items := extractUTXOTransfers(raw, watchAddress)
		return ports.IncomingTransferResult{Items: items}
	}
	items, next := extractAccountTransfers(chain, raw, watchAddress)
	return ports.IncomingTransferResult{Items: items, NextCursor: next}
}

func Finality(txHash string, raw map[string]any) ports.TxFinality {
	foundHash := findFirstString(raw, "tx_hash", "txid", "txId", "hash")
	if strings.TrimSpace(txHash) == "" {
		txHash = foundHash
	}
	confirmations := findFirstInt64(raw, "confirmations", "confirmation", "confirm")
	status := normalizeTxStatus(findFirstString(raw, "status", "tx_status", "state", "receipt_status"))

	found := false
	if txHash != "" || foundHash != "" {
		found = true
	}
	if len(raw) > 0 {
		found = true
	}
	if strings.EqualFold(status, "REVERTED") {
		found = true
	}
	if status == "" {
		if confirmations > 0 {
			status = "CONFIRMED"
		} else if found {
			status = "PENDING"
		}
	}

	return ports.TxFinality{
		TxHash:        txHash,
		Confirmations: normalizeConfirmations(confirmations),
		Status:        status,
		Found:         found,
	}
}

func extractAccountTransfers(chain string, raw map[string]any, watchAddress string) ([]ports.IncomingTransfer, string) {
	switch normalizeChain(chain) {
	case "ethereum", "binance", "polygon", "arbitrum", "optimism", "linea", "scroll", "mantle", "zksync":
		if txs, next, ok := extractAccountTransfersEVM(raw, watchAddress); ok {
			return txs, next
		}
	case "tron":
		if txs, next, ok := extractAccountTransfersTRON(raw, watchAddress); ok {
			return txs, next
		}
	case "solana":
		if txs, next, ok := extractAccountTransfersSOL(raw, watchAddress); ok {
			return txs, next
		}
	}
	return extractAccountTransfersGeneric(raw, watchAddress)
}

func extractAccountTransfersEVM(raw map[string]any, watchAddress string) ([]ports.IncomingTransfer, string, bool) {
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, "", false
	}
	type addressNode struct {
		Address string `json:"address"`
	}
	type valueNode struct {
		Value string `json:"value"`
	}
	type txNode struct {
		Hash          string        `json:"hash"`
		Index         int64         `json:"index"`
		Froms         []addressNode `json:"froms"`
		Tos           []addressNode `json:"tos"`
		Values        []valueNode   `json:"values"`
		Confirmations int64         `json:"confirmations"`
		Status        string        `json:"status"`
	}
	type resp struct {
		Tx []txNode `json:"tx"`
	}
	var parsed resp
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, "", false
	}
	if len(parsed.Tx) == 0 {
		return nil, "", false
	}
	watch := strings.ToLower(strings.TrimSpace(watchAddress))
	out := make([]ports.IncomingTransfer, 0, len(parsed.Tx))
	for i, tx := range parsed.Tx {
		if strings.TrimSpace(tx.Hash) == "" {
			continue
		}
		to := ""
		if len(tx.Tos) > 0 {
			to = strings.TrimSpace(tx.Tos[0].Address)
		}
		from := ""
		if len(tx.Froms) > 0 {
			from = strings.TrimSpace(tx.Froms[0].Address)
		}
		if watch != "" && strings.ToLower(to) != watch {
			continue
		}
		amount := "0"
		if len(tx.Values) > 0 && strings.TrimSpace(tx.Values[0].Value) != "" {
			amount = strings.TrimSpace(tx.Values[0].Value)
		}
		idx := tx.Index
		if idx <= 0 {
			idx = int64(i)
		}
		out = append(out, ports.IncomingTransfer{
			TxHash:        strings.TrimSpace(tx.Hash),
			FromAddress:   from,
			ToAddress:     fallback(to, watchAddress),
			Amount:        amount,
			Confirmations: normalizeConfirmations(tx.Confirmations),
			Index:         idx,
			Status:        normalizeTxStatus(tx.Status),
		})
	}
	next := findFirstString(raw, "next_cursor", "nextCursor", "cursor", "next")
	return out, next, true
}

func extractAccountTransfersTRON(raw map[string]any, watchAddress string) ([]ports.IncomingTransfer, string, bool) {
	// Current TRON payload in legacy adapters follows EVM-like tx[].froms/tos/values.
	return extractAccountTransfersEVM(raw, watchAddress)
}

func extractAccountTransfersSOL(raw map[string]any, watchAddress string) ([]ports.IncomingTransfer, string, bool) {
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, "", false
	}
	type txNode struct {
		Hash          string `json:"hash"`
		To            string `json:"to"`
		From          string `json:"from"`
		Value         string `json:"value"`
		Status        string `json:"status"`
		Index         int64  `json:"index"`
		Confirmations int64  `json:"confirmations"`
	}
	type resp struct {
		Tx []txNode `json:"tx"`
	}
	var parsed resp
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, "", false
	}
	if len(parsed.Tx) == 0 {
		return nil, "", false
	}
	watch := strings.ToLower(strings.TrimSpace(watchAddress))
	out := make([]ports.IncomingTransfer, 0, len(parsed.Tx))
	for i, tx := range parsed.Tx {
		if strings.TrimSpace(tx.Hash) == "" {
			continue
		}
		to := strings.TrimSpace(tx.To)
		if watch != "" && to != "" && strings.ToLower(to) != watch {
			continue
		}
		idx := tx.Index
		if idx <= 0 {
			idx = int64(i)
		}
		amount := strings.TrimSpace(tx.Value)
		if amount == "" {
			amount = "0"
		}
		out = append(out, ports.IncomingTransfer{
			TxHash:        tx.Hash,
			FromAddress:   tx.From,
			ToAddress:     fallback(to, watchAddress),
			Amount:        amount,
			Confirmations: normalizeConfirmations(tx.Confirmations),
			Index:         idx,
			Status:        normalizeTxStatus(tx.Status),
		})
	}
	next := findFirstString(raw, "next_cursor", "nextCursor", "cursor", "next")
	return out, next, true
}

func extractAccountTransfersGeneric(raw map[string]any, watchAddress string) ([]ports.IncomingTransfer, string) {
	objects := collectObjectCandidates(raw)
	out := make([]ports.IncomingTransfer, 0, len(objects))
	watchAddress = strings.ToLower(strings.TrimSpace(watchAddress))
	for idx, obj := range objects {
		txHash := pickString(obj, "tx_hash", "hash", "txid", "txId")
		if txHash == "" {
			continue
		}
		to := pickString(obj, "to", "to_address", "toAddress", "recipient", "receiver")
		from := pickString(obj, "from", "from_address", "fromAddress", "sender")
		amount := pickString(obj, "amount", "value", "amount_str", "amountStr")
		if amount == "" {
			amount = "0"
		}
		if watchAddress != "" && strings.ToLower(strings.TrimSpace(to)) != "" && strings.ToLower(strings.TrimSpace(to)) != watchAddress {
			continue
		}
		confirmations := pickInt64(obj, "confirmations", "confirm", "confirmation")
		eventIdx := pickInt64(obj, "index", "vout", "n")
		if eventIdx <= 0 {
			eventIdx = int64(idx)
		}
		out = append(out, ports.IncomingTransfer{
			TxHash:        txHash,
			FromAddress:   from,
			ToAddress:     fallback(to, watchAddress),
			Amount:        amount,
			Confirmations: normalizeConfirmations(confirmations),
			Index:         eventIdx,
			Status:        normalizeTxStatus(pickString(obj, "status", "tx_status", "state")),
		})
	}
	next := findFirstString(raw, "next_cursor", "nextCursor", "cursor", "next")
	return out, next
}

func extractUTXOTransfers(raw map[string]any, watchAddress string) []ports.IncomingTransfer {
	objects := collectObjectCandidates(raw)
	out := make([]ports.IncomingTransfer, 0, len(objects))
	for _, obj := range objects {
		txHash := pickString(obj, "tx_hash", "hash", "txid", "txId")
		if txHash == "" {
			continue
		}
		idx := pickInt64(obj, "index", "vout", "tx_out_n", "txOutN")
		amount := pickString(obj, "amount", "unspent_amount", "value", "unspentAmount")
		if amount == "" {
			amount = "0"
		}
		address := pickString(obj, "address", "to", "to_address")
		if address == "" {
			address = watchAddress
		}
		conf := pickInt64(obj, "confirmations", "confirm", "confirmation")
		out = append(out, ports.IncomingTransfer{
			TxHash:        txHash,
			FromAddress:   "",
			ToAddress:     address,
			Amount:        amount,
			Confirmations: conf,
			Index:         idx,
			Status:        "",
		})
	}
	return out
}

func collectObjectCandidates(v any) []map[string]any {
	out := make([]map[string]any, 0, 16)
	var walk func(any)
	walk = func(x any) {
		switch vv := x.(type) {
		case map[string]any:
			if pickString(vv, "tx_hash", "hash", "txid", "txId") != "" {
				out = append(out, vv)
			}
			for _, child := range vv {
				walk(child)
			}
		case []any:
			for _, child := range vv {
				walk(child)
			}
		}
	}
	walk(v)
	return out
}

func pickString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch vv := v.(type) {
			case string:
				if strings.TrimSpace(vv) != "" {
					return strings.TrimSpace(vv)
				}
			case json.Number:
				return vv.String()
			case float64:
				return strconv.FormatInt(int64(vv), 10)
			}
		}
	}
	return ""
}

func pickInt64(m map[string]any, keys ...string) int64 {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch vv := v.(type) {
			case float64:
				return int64(vv)
			case json.Number:
				n, _ := vv.Int64()
				return n
			case string:
				n, _ := strconv.ParseInt(strings.TrimSpace(vv), 10, 64)
				return n
			}
		}
	}
	return 0
}

func findFirstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				return strings.TrimSpace(s)
			}
		}
	}
	for _, v := range m {
		switch vv := v.(type) {
		case map[string]any:
			s := findFirstString(vv, keys...)
			if s != "" {
				return s
			}
		case []any:
			for _, item := range vv {
				if m2, ok := item.(map[string]any); ok {
					s := findFirstString(m2, keys...)
					if s != "" {
						return s
					}
				}
			}
		}
	}
	return ""
}

func findFirstInt64(m map[string]any, keys ...string) int64 {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch vv := v.(type) {
			case float64:
				return int64(vv)
			case int64:
				return vv
			case int:
				return int64(vv)
			case json.Number:
				n, err := vv.Int64()
				if err == nil {
					return n
				}
			case string:
				n, err := strconv.ParseInt(strings.TrimSpace(vv), 10, 64)
				if err == nil {
					return n
				}
			}
		}
	}
	for _, v := range m {
		switch vv := v.(type) {
		case map[string]any:
			if n := findFirstInt64(vv, keys...); n != 0 {
				return n
			}
		case []any:
			for _, item := range vv {
				if m2, ok := item.(map[string]any); ok {
					if n := findFirstInt64(m2, keys...); n != 0 {
						return n
					}
				}
			}
		}
	}
	return 0
}

func fallback(v, d string) string {
	if strings.TrimSpace(v) == "" {
		return d
	}
	return v
}

func normalizeChain(chain string) string {
	x := strings.ToLower(strings.TrimSpace(chain))
	switch x {
	case "eth":
		return "ethereum"
	case "trx":
		return "tron"
	case "sol":
		return "solana"
	case "bsc":
		return "binance"
	default:
		return x
	}
}

func normalizeTxStatus(v string) string {
	x := strings.ToLower(strings.TrimSpace(v))
	switch x {
	case "success", "succeeded", "ok", "confirmed":
		return "CONFIRMED"
	case "failed", "fail", "reverted", "dropped":
		return "REVERTED"
	case "pending":
		return "PENDING"
	default:
		return ""
	}
}

func normalizeConfirmations(v int64) int64 {
	if v < 0 {
		return -1
	}
	if v == 0 {
		// Keep unknown/zero as sentinel for cautious upper-layer handling.
		return -1
	}
	return v
}
