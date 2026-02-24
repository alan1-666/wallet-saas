package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type ChainGateway struct {
	baseURL string
	http    *http.Client
}

type AccountTx struct {
	TxHash        string
	FromAddress   string
	ToAddress     string
	Amount        string
	Confirmations int64
	Index         int64
	Status        string
}

type UTXO struct {
	TxHash        string
	Index         int64
	Address       string
	Amount        string
	Confirmations int64
}

func NewChainGateway(baseURL string) *ChainGateway {
	return &ChainGateway{
		baseURL: strings.TrimRight(baseURL, "/"),
		http: &http.Client{
			Timeout: 12 * time.Second,
		},
	}
}

func (c *ChainGateway) TxByAddress(ctx context.Context, chain, coin, network, address, cursor string, pageSize int) ([]AccountTx, string, error) {
	reqBody := map[string]any{
		"chain":     chain,
		"coin":      coin,
		"network":   network,
		"address":   address,
		"page":      1,
		"page_size": pageSize,
		"cursor":    cursor,
	}
	raw, err := c.postJSON(ctx, "/v1/chain/tx-by-address", reqBody)
	if err != nil {
		return nil, "", err
	}
	txs, next := extractAccountTxs(chain, raw, address)
	return txs, next, nil
}

func (c *ChainGateway) UnspentOutputs(ctx context.Context, chain, network, address string) ([]UTXO, error) {
	reqBody := map[string]any{
		"chain":   chain,
		"network": network,
		"address": address,
	}
	raw, err := c.postJSON(ctx, "/v1/chain/unspent-outputs", reqBody)
	if err != nil {
		return nil, err
	}
	return extractUTXOs(raw, address), nil
}

func (c *ChainGateway) postJSON(ctx context.Context, path string, body any) (map[string]any, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("chain-gateway %s failed: %s", path, resp.Status)
	}
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

func extractAccountTxs(chain string, raw map[string]any, watchAddress string) ([]AccountTx, string) {
	switch normalizeChain(chain) {
	case "ethereum", "eth", "binance", "bsc", "polygon", "arbitrum", "optimism", "linea", "scroll", "mantle", "zksync":
		if txs, next, ok := extractAccountTxsEVM(raw, watchAddress); ok {
			return txs, next
		}
	case "tron", "trx":
		if txs, next, ok := extractAccountTxsTRON(raw, watchAddress); ok {
			return txs, next
		}
	case "solana", "sol":
		if txs, next, ok := extractAccountTxsSOL(raw, watchAddress); ok {
			return txs, next
		}
	}
	return extractAccountTxsGeneric(raw, watchAddress)
}

func extractAccountTxsEVM(raw map[string]any, watchAddress string) ([]AccountTx, string, bool) {
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
		Height        string        `json:"height"`
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
	out := make([]AccountTx, 0, len(parsed.Tx))
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
		out = append(out, AccountTx{
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

func extractAccountTxsTRON(raw map[string]any, watchAddress string) ([]AccountTx, string, bool) {
	// TRON response shape in legacy tests is same as EVM: tx[].froms/tos/values/status.
	return extractAccountTxsEVM(raw, watchAddress)
}

func extractAccountTxsSOL(raw map[string]any, watchAddress string) ([]AccountTx, string, bool) {
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
	out := make([]AccountTx, 0, len(parsed.Tx))
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
		out = append(out, AccountTx{
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

func extractAccountTxsGeneric(raw map[string]any, watchAddress string) ([]AccountTx, string) {
	objects := collectObjectCandidates(raw)
	out := make([]AccountTx, 0, len(objects))
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
		out = append(out, AccountTx{
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

func extractUTXOs(raw map[string]any, watchAddress string) []UTXO {
	objects := collectObjectCandidates(raw)
	out := make([]UTXO, 0, len(objects))
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
		out = append(out, UTXO{
			TxHash:        txHash,
			Index:         idx,
			Address:       address,
			Amount:        amount,
			Confirmations: conf,
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

func fallback(v, d string) string {
	if strings.TrimSpace(v) == "" {
		return d
	}
	return v
}

func normalizeChain(chain string) string {
	return strings.ToLower(strings.TrimSpace(chain))
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
		// 0 can mean unknown in many chain adapters; keep as unknown sentinel.
		return -1
	}
	return v
}
