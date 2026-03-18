package solana

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"wallet-saas-v2/services/chain-gateway/internal/endpoint"
	"wallet-saas-v2/services/chain-gateway/internal/ports"
)

var ErrNoEndpoint = errors.New("no rpc endpoint configured")

type RPCReader struct {
	Endpoints   *endpoint.Manager
	HTTPClient  *http.Client
	FallbackURL string
}

func NewRPCReader(endpoints *endpoint.Manager, fallbackURL string) *RPCReader {
	return &RPCReader{
		Endpoints:   endpoints,
		HTTPClient:  &http.Client{Timeout: 12 * time.Second},
		FallbackURL: strings.TrimSpace(fallbackURL),
	}
}

func (r *RPCReader) GetBalance(ctx context.Context, chain, network, address string) (ports.BalanceResult, error) {
	ep, err := r.selectEndpoint(chain, network)
	if err != nil {
		return ports.BalanceResult{}, err
	}
	out := solanaBalanceResult{}
	if err := r.call(ctx, ep, "getBalance", []any{
		strings.TrimSpace(address),
		map[string]any{"commitment": "confirmed"},
	}, &out); err != nil {
		r.ReportFailure(ep.Key, err)
		return ports.BalanceResult{}, err
	}
	r.ReportSuccess(ep.Key)
	return ports.BalanceResult{
		Balance: strconv.FormatInt(out.Value, 10),
		Network: strings.ToLower(strings.TrimSpace(network)),
	}, nil
}

func (r *RPCReader) GetTokenBalance(ctx context.Context, chain, network, ownerAddress, mintAddress string) (ports.BalanceResult, error) {
	ep, err := r.selectEndpoint(chain, network)
	if err != nil {
		return ports.BalanceResult{}, err
	}
	out := solanaTokenAccountsByOwnerResult{}
	if err := r.call(ctx, ep, "getTokenAccountsByOwner", []any{
		strings.TrimSpace(ownerAddress),
		map[string]any{"mint": strings.TrimSpace(mintAddress)},
		map[string]any{"encoding": "jsonParsed", "commitment": "confirmed"},
	}, &out); err != nil {
		r.ReportFailure(ep.Key, err)
		return ports.BalanceResult{}, err
	}
	r.ReportSuccess(ep.Key)
	balance, err := out.totalAmount()
	if err != nil {
		return ports.BalanceResult{}, err
	}
	return ports.BalanceResult{
		Balance: balance,
		Network: strings.ToLower(strings.TrimSpace(network)),
	}, nil
}

func (r *RPCReader) GetTxFinality(ctx context.Context, chain, network, txHash string) (ports.TxFinality, error) {
	ep, err := r.selectEndpoint(chain, network)
	if err != nil {
		return ports.TxFinality{}, err
	}
	latestSlot, err := r.getLatestSlot(ctx, ep)
	if err != nil {
		r.ReportFailure(ep.Key, err)
		return ports.TxFinality{}, err
	}
	status, found, err := r.getSignatureStatus(ctx, ep, txHash)
	if err != nil {
		r.ReportFailure(ep.Key, err)
		return ports.TxFinality{}, err
	}
	if !found {
		r.ReportSuccess(ep.Key)
		return ports.TxFinality{
			TxHash: strings.TrimSpace(txHash),
			Status: "PENDING",
			Found:  false,
		}, nil
	}
	confirmations := int64(0)
	if latestSlot >= status.Slot {
		confirmations = int64(latestSlot-status.Slot) + 1
	}
	statusText := "PENDING"
	if status.Err != nil {
		statusText = "REVERTED"
	} else if confirmations > 0 || status.ConfirmationStatus == "confirmed" || status.ConfirmationStatus == "finalized" {
		statusText = "CONFIRMED"
	}
	r.ReportSuccess(ep.Key)
	return ports.TxFinality{
		TxHash:        strings.TrimSpace(txHash),
		Confirmations: confirmations,
		Status:        statusText,
		Found:         true,
	}, nil
}

func (r *RPCReader) ListIncomingTransfers(ctx context.Context, chain, network, address, cursor string, pageSize uint32) (ports.IncomingTransferResult, error) {
	ep, err := r.selectEndpoint(chain, network)
	if err != nil {
		return ports.IncomingTransferResult{}, err
	}
	if strings.TrimSpace(address) == "" {
		return ports.IncomingTransferResult{}, fmt.Errorf("address is required")
	}
	if pageSize == 0 {
		pageSize = 50
	}
	if pageSize > 200 {
		pageSize = 200
	}

	state := parseCursor(cursor)
	sigs, err := r.getSignaturesForAddress(ctx, ep, strings.TrimSpace(address), int(pageSize), state.Before, state.Until)
	if err != nil {
		r.ReportFailure(ep.Key, err)
		return ports.IncomingTransferResult{}, err
	}
	if len(sigs) == 0 {
		r.ReportSuccess(ep.Key)
		return ports.IncomingTransferResult{Items: nil, NextCursor: state.watermark()}, nil
	}
	if state.Head == "" {
		state.Head = sigs[0].Signature
	}

	latestSlot, err := r.getLatestSlot(ctx, ep)
	if err != nil {
		r.ReportFailure(ep.Key, err)
		return ports.IncomingTransferResult{}, err
	}

	watch := strings.TrimSpace(address)
	items := make([]ports.IncomingTransfer, 0, len(sigs))
	for idx, sig := range sigs {
		tx, found, err := r.getTransaction(ctx, ep, sig.Signature)
		if err != nil {
			r.ReportFailure(ep.Key, err)
			return ports.IncomingTransferResult{}, err
		}
		if !found {
			continue
		}
		transfer, ok := buildIncomingTransferFromTx(tx, watch, sig, latestSlot, idx)
		if !ok {
			continue
		}
		items = append(items, transfer)
	}

	next := state.Head
	if len(sigs) == int(pageSize) {
		next = encodeCursor(cursorState{Head: state.Head, Until: state.Until, Before: sigs[len(sigs)-1].Signature})
	}
	r.ReportSuccess(ep.Key)
	return ports.IncomingTransferResult{Items: items, NextCursor: next}, nil
}

func buildIncomingTransferFromTx(tx solanaTx, watch string, sig solanaSignatureInfo, latestSlot uint64, idx int) (ports.IncomingTransfer, bool) {
	keys := tx.accountKeys()
	if len(keys) == 0 {
		return ports.IncomingTransfer{}, false
	}
	targetIdx := -1
	for i, k := range keys {
		if k == watch {
			targetIdx = i
			break
		}
	}
	if targetIdx < 0 {
		return ports.IncomingTransfer{}, false
	}
	pre := tx.preBalance(targetIdx)
	post := tx.postBalance(targetIdx)
	if post <= pre {
		return ports.IncomingTransfer{}, false
	}
	amount := post - pre
	confirmations := int64(0)
	if latestSlot >= sig.Slot {
		confirmations = int64(latestSlot-sig.Slot) + 1
	}
	status := "PENDING"
	if sig.Err != nil || tx.metaErr() {
		status = "REVERTED"
	} else if confirmations > 0 || sig.ConfirmationStatus == "confirmed" || sig.ConfirmationStatus == "finalized" {
		status = "CONFIRMED"
	}
	fromAddr := ""
	if len(keys) > 0 {
		fromAddr = keys[0]
	}
	return ports.IncomingTransfer{
		TxHash:        sig.Signature,
		FromAddress:   fromAddr,
		ToAddress:     watch,
		Amount:        strconv.FormatUint(amount, 10),
		Confirmations: confirmations,
		Index:         int64(idx),
		Status:        status,
	}, true
}

func (r *RPCReader) selectEndpoint(chain, network string) (endpoint.SelectedEndpoint, error) {
	if r == nil {
		return endpoint.SelectedEndpoint{}, ErrNoEndpoint
	}
	if r.Endpoints != nil {
		ep, err := r.Endpoints.Select(chain, network, string(ports.ModelAccount))
		if err == nil {
			return ep, nil
		}
		if strings.TrimSpace(r.FallbackURL) == "" {
			return endpoint.SelectedEndpoint{}, fmt.Errorf("%w: %v", ErrNoEndpoint, err)
		}
	}
	if strings.TrimSpace(r.FallbackURL) != "" {
		return endpoint.SelectedEndpoint{
			Key:       "solana-fallback-rpc",
			Chain:     strings.ToLower(strings.TrimSpace(chain)),
			Network:   strings.ToLower(strings.TrimSpace(network)),
			Model:     string(ports.ModelAccount),
			URL:       strings.TrimSpace(r.FallbackURL),
			TimeoutMS: 12000,
		}, nil
	}
	return endpoint.SelectedEndpoint{}, ErrNoEndpoint
}

func (r *RPCReader) ReportFailure(endpointKey string, reason error) {
	if r == nil || r.Endpoints == nil {
		return
	}
	r.Endpoints.ReportFailure(endpointKey, reason)
}

func (r *RPCReader) ReportSuccess(endpointKey string) {
	if r == nil || r.Endpoints == nil {
		return
	}
	r.Endpoints.ReportSuccess(endpointKey)
}

func (r *RPCReader) getLatestSlot(ctx context.Context, ep endpoint.SelectedEndpoint) (uint64, error) {
	result := uint64(0)
	if err := r.call(ctx, ep, "getSlot", []any{map[string]any{"commitment": "processed"}}, &result); err != nil {
		return 0, err
	}
	return result, nil
}

func (r *RPCReader) getSignatureStatus(ctx context.Context, ep endpoint.SelectedEndpoint, signature string) (solanaSignatureStatus, bool, error) {
	out := solanaSignatureStatusResult{}
	if err := r.call(ctx, ep, "getSignatureStatuses", []any{
		[]string{strings.TrimSpace(signature)},
		map[string]any{"searchTransactionHistory": true},
	}, &out); err != nil {
		return solanaSignatureStatus{}, false, err
	}
	if len(out.Value) == 0 || out.Value[0] == nil {
		return solanaSignatureStatus{}, false, nil
	}
	return *out.Value[0], true, nil
}

func (r *RPCReader) getSignaturesForAddress(ctx context.Context, ep endpoint.SelectedEndpoint, address string, limit int, before, until string) ([]solanaSignatureInfo, error) {
	opts := map[string]any{"limit": limit}
	if strings.TrimSpace(before) != "" {
		opts["before"] = strings.TrimSpace(before)
	}
	if strings.TrimSpace(until) != "" {
		opts["until"] = strings.TrimSpace(until)
	}
	out := make([]solanaSignatureInfo, 0, limit)
	if err := r.call(ctx, ep, "getSignaturesForAddress", []any{address, opts}, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *RPCReader) getTransaction(ctx context.Context, ep endpoint.SelectedEndpoint, signature string) (solanaTx, bool, error) {
	var raw json.RawMessage
	if err := r.call(ctx, ep, "getTransaction", []any{
		strings.TrimSpace(signature),
		map[string]any{"encoding": "json", "maxSupportedTransactionVersion": 0, "commitment": "confirmed"},
	}, &raw); err != nil {
		return solanaTx{}, false, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return solanaTx{}, false, nil
	}
	var tx solanaTx
	if err := json.Unmarshal(raw, &tx); err != nil {
		return solanaTx{}, false, err
	}
	return tx, true, nil
}

func (r *RPCReader) call(ctx context.Context, ep endpoint.SelectedEndpoint, method string, params []any, out any) error {
	timeout := 10 * time.Second
	if ep.TimeoutMS > 0 {
		timeout = time.Duration(ep.TimeoutMS) * time.Millisecond
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  params,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	client := r.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, ep.URL, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("rpc status=%d method=%s", resp.StatusCode, method)
	}
	var rpcResp solanaRPCEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return err
	}
	if rpcResp.Error != nil {
		return fmt.Errorf("rpc method=%s code=%d msg=%s", method, rpcResp.Error.Code, rpcResp.Error.Message)
	}
	if out == nil {
		return nil
	}
	if len(rpcResp.Result) == 0 {
		return nil
	}
	return json.Unmarshal(rpcResp.Result, out)
}

type solanaRPCEnvelope struct {
	Result json.RawMessage `json:"result"`
	Error  *solanaRPCError `json:"error"`
}

type solanaRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type solanaBalanceResult struct {
	Value int64 `json:"value"`
}

type solanaTokenAccountsByOwnerResult struct {
	Value []solanaTokenAccountEntry `json:"value"`
}

type solanaTokenAccountEntry struct {
	Account struct {
		Data struct {
			Parsed struct {
				Info struct {
					TokenAmount struct {
						Amount string `json:"amount"`
					} `json:"tokenAmount"`
				} `json:"info"`
			} `json:"parsed"`
		} `json:"data"`
	} `json:"account"`
}

func (r solanaTokenAccountsByOwnerResult) totalAmount() (string, error) {
	total := new(big.Int)
	for _, item := range r.Value {
		amount := strings.TrimSpace(item.Account.Data.Parsed.Info.TokenAmount.Amount)
		if amount == "" {
			continue
		}
		value, ok := new(big.Int).SetString(amount, 10)
		if !ok {
			return "", fmt.Errorf("invalid token amount: %s", amount)
		}
		total.Add(total, value)
	}
	return total.String(), nil
}

type solanaSignatureStatusResult struct {
	Value []*solanaSignatureStatus `json:"value"`
}

type solanaSignatureStatus struct {
	Slot               uint64         `json:"slot"`
	Confirmations      *uint64        `json:"confirmations"`
	Err                map[string]any `json:"err"`
	ConfirmationStatus string         `json:"confirmationStatus"`
}

type solanaSignatureInfo struct {
	Signature          string         `json:"signature"`
	Slot               uint64         `json:"slot"`
	Err                map[string]any `json:"err"`
	ConfirmationStatus string         `json:"confirmationStatus"`
}

type solanaTx struct {
	Slot        uint64 `json:"slot"`
	Transaction struct {
		Message struct {
			AccountKeys []any `json:"accountKeys"`
		} `json:"message"`
	} `json:"transaction"`
	Meta struct {
		PreBalances  []uint64       `json:"preBalances"`
		PostBalances []uint64       `json:"postBalances"`
		Err          map[string]any `json:"err"`
	} `json:"meta"`
}

func (t solanaTx) accountKeys() []string {
	out := make([]string, 0, len(t.Transaction.Message.AccountKeys))
	for _, item := range t.Transaction.Message.AccountKeys {
		switch v := item.(type) {
		case string:
			if strings.TrimSpace(v) != "" {
				out = append(out, strings.TrimSpace(v))
			}
		case map[string]any:
			if pub, ok := v["pubkey"].(string); ok && strings.TrimSpace(pub) != "" {
				out = append(out, strings.TrimSpace(pub))
			}
		}
	}
	return out
}

func (t solanaTx) preBalance(idx int) uint64 {
	if idx < 0 || idx >= len(t.Meta.PreBalances) {
		return 0
	}
	return t.Meta.PreBalances[idx]
}

func (t solanaTx) postBalance(idx int) uint64 {
	if idx < 0 || idx >= len(t.Meta.PostBalances) {
		return 0
	}
	return t.Meta.PostBalances[idx]
}

func (t solanaTx) metaErr() bool {
	return t.Meta.Err != nil
}

type cursorState struct {
	Head   string
	Until  string
	Before string
}

func parseCursor(cursor string) cursorState {
	v := strings.TrimSpace(cursor)
	if v == "" {
		return cursorState{}
	}
	if !strings.HasPrefix(v, "v1|") {
		return cursorState{Head: v, Until: v}
	}
	parts := strings.SplitN(v, "|", 4)
	if len(parts) != 4 {
		return cursorState{Head: v, Until: v}
	}
	return cursorState{
		Head:   strings.TrimSpace(parts[1]),
		Until:  strings.TrimSpace(parts[2]),
		Before: strings.TrimSpace(parts[3]),
	}
}

func encodeCursor(c cursorState) string {
	return "v1|" + strings.TrimSpace(c.Head) + "|" + strings.TrimSpace(c.Until) + "|" + strings.TrimSpace(c.Before)
}

func (c cursorState) watermark() string {
	if strings.TrimSpace(c.Head) != "" {
		return strings.TrimSpace(c.Head)
	}
	return strings.TrimSpace(c.Until)
}
