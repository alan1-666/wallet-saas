package evm

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

const defaultCursorLookback uint64 = 256

var ErrNoEndpoint = errors.New("no rpc endpoint configured")

type RPCReader struct {
	Endpoints      *endpoint.Manager
	HTTPClient     *http.Client
	CursorLookback uint64
}

func NewRPCReader(endpoints *endpoint.Manager) *RPCReader {
	return &RPCReader{
		Endpoints:      endpoints,
		HTTPClient:     &http.Client{Timeout: 15 * time.Second},
		CursorLookback: defaultCursorLookback,
	}
}

func (r *RPCReader) ListIncomingTransfers(ctx context.Context, chain, network, address, cursor string, pageSize uint32) (ports.IncomingTransferResult, error) {
	ep, err := r.selectEndpoint(chain, network)
	if err != nil {
		return ports.IncomingTransferResult{}, err
	}
	if pageSize == 0 {
		pageSize = 50
	}
	if pageSize > 500 {
		pageSize = 500
	}
	addr := strings.ToLower(strings.TrimSpace(address))
	filterByAddress := addr != ""

	latest, err := r.blockNumber(ctx, ep)
	if err != nil {
		r.Endpoints.ReportFailure(ep.Key, err)
		return ports.IncomingTransferResult{}, err
	}

	from, err := r.resolveFromBlock(cursor, latest)
	if err != nil {
		r.Endpoints.ReportFailure(ep.Key, err)
		return ports.IncomingTransferResult{}, err
	}
	if from > latest {
		r.Endpoints.ReportSuccess(ep.Key)
		return ports.IncomingTransferResult{
			Items:      []ports.IncomingTransfer{},
			NextCursor: strconv.FormatUint(latest, 10),
		}, nil
	}
	to := from + uint64(pageSize) - 1
	if to > latest {
		to = latest
	}

	items := make([]ports.IncomingTransfer, 0, pageSize)
	for blockNum := from; blockNum <= to; blockNum++ {
		block, err := r.getBlockByNumber(ctx, ep, blockNum)
		if err != nil {
			r.Endpoints.ReportFailure(ep.Key, err)
			return ports.IncomingTransferResult{}, err
		}
		for _, tx := range block.Transactions {
			if strings.TrimSpace(tx.Hash) == "" {
				continue
			}
			toAddr := strings.ToLower(strings.TrimSpace(tx.To))
			if toAddr == "" {
				continue
			}
			if filterByAddress && toAddr != addr {
				continue
			}
			amt, err := hexToBig(tx.Value)
			if err != nil || amt.Sign() <= 0 {
				continue
			}
			receipt, err := r.getTransactionReceipt(ctx, ep, tx.Hash)
			if err != nil {
				r.Endpoints.ReportFailure(ep.Key, err)
				return ports.IncomingTransferResult{}, err
			}
			conf := int64(0)
			if receipt.BlockNumber != "" {
				bn, err := hexToUint64(receipt.BlockNumber)
				if err == nil && latest >= bn {
					conf = int64(latest-bn) + 1
				}
			}
			status := "PENDING"
			switch strings.ToLower(strings.TrimSpace(receipt.Status)) {
			case "0x1":
				status = "CONFIRMED"
			case "0x0":
				status = "REVERTED"
			default:
				if conf > 0 {
					status = "CONFIRMED"
				}
			}
			txIndex, _ := hexToUint64(tx.TransactionIndex)
			items = append(items, ports.IncomingTransfer{
				TxHash:        tx.Hash,
				FromAddress:   tx.From,
				ToAddress:     tx.To,
				Amount:        amt.String(),
				Confirmations: conf,
				Index:         int64(txIndex),
				Status:        status,
			})
		}
	}

	r.Endpoints.ReportSuccess(ep.Key)
	return ports.IncomingTransferResult{
		Items:      items,
		NextCursor: strconv.FormatUint(to, 10),
	}, nil
}

func (r *RPCReader) GetTxFinality(ctx context.Context, chain, network, txHash string) (ports.TxFinality, error) {
	ep, err := r.selectEndpoint(chain, network)
	if err != nil {
		return ports.TxFinality{}, err
	}
	tx, found, err := r.getTransactionByHash(ctx, ep, txHash)
	if err != nil {
		r.Endpoints.ReportFailure(ep.Key, err)
		return ports.TxFinality{}, err
	}
	if !found {
		r.Endpoints.ReportSuccess(ep.Key)
		return ports.TxFinality{
			TxHash: strings.TrimSpace(txHash),
			Found:  false,
			Status: "PENDING",
		}, nil
	}

	if strings.TrimSpace(tx.BlockNumber) == "" {
		r.Endpoints.ReportSuccess(ep.Key)
		return ports.TxFinality{
			TxHash:        fallbackHash(tx.Hash, txHash),
			Confirmations: 0,
			Status:        "PENDING",
			Found:         true,
		}, nil
	}

	blockNum, err := hexToUint64(tx.BlockNumber)
	if err != nil {
		r.Endpoints.ReportFailure(ep.Key, err)
		return ports.TxFinality{}, err
	}
	latest, err := r.blockNumber(ctx, ep)
	if err != nil {
		r.Endpoints.ReportFailure(ep.Key, err)
		return ports.TxFinality{}, err
	}
	receipt, err := r.getTransactionReceipt(ctx, ep, txHash)
	if err != nil {
		r.Endpoints.ReportFailure(ep.Key, err)
		return ports.TxFinality{}, err
	}
	status := "PENDING"
	switch strings.ToLower(strings.TrimSpace(receipt.Status)) {
	case "0x1":
		status = "CONFIRMED"
	case "0x0":
		status = "REVERTED"
	default:
		if latest >= blockNum {
			status = "CONFIRMED"
		}
	}
	confirmations := int64(0)
	if latest >= blockNum {
		confirmations = int64(latest-blockNum) + 1
	}
	r.Endpoints.ReportSuccess(ep.Key)
	return ports.TxFinality{
		TxHash:        fallbackHash(tx.Hash, txHash),
		Confirmations: confirmations,
		Status:        status,
		Found:         true,
	}, nil
}

func (r *RPCReader) GetBalance(ctx context.Context, chain, network, address string) (ports.BalanceResult, error) {
	ep, err := r.selectEndpoint(chain, network)
	if err != nil {
		return ports.BalanceResult{}, err
	}
	result := ""
	if err := r.call(ctx, ep, "eth_getBalance", []any{address, "latest"}, &result); err != nil {
		r.Endpoints.ReportFailure(ep.Key, err)
		return ports.BalanceResult{}, err
	}
	bal, err := hexToBig(result)
	if err != nil {
		r.Endpoints.ReportFailure(ep.Key, err)
		return ports.BalanceResult{}, err
	}
	r.Endpoints.ReportSuccess(ep.Key)
	return ports.BalanceResult{Balance: bal.String(), Network: strings.ToLower(strings.TrimSpace(network))}, nil
}

func (r *RPCReader) SelectEndpoint(chain, network string) (endpoint.SelectedEndpoint, error) {
	return r.selectEndpoint(chain, network)
}

func (r *RPCReader) ReportFailure(endpointKey string, reason error) {
	if r != nil && r.Endpoints != nil {
		r.Endpoints.ReportFailure(endpointKey, reason)
	}
}

func (r *RPCReader) ReportSuccess(endpointKey string) {
	if r != nil && r.Endpoints != nil {
		r.Endpoints.ReportSuccess(endpointKey)
	}
}

func (r *RPCReader) selectEndpoint(chain, network string) (endpoint.SelectedEndpoint, error) {
	if r == nil || r.Endpoints == nil {
		return endpoint.SelectedEndpoint{}, ErrNoEndpoint
	}
	ep, err := r.Endpoints.Select(chain, network, string(ports.ModelAccount))
	if err != nil {
		return endpoint.SelectedEndpoint{}, fmt.Errorf("%w: %v", ErrNoEndpoint, err)
	}
	return ep, nil
}

func (r *RPCReader) resolveFromBlock(cursor string, latest uint64) (uint64, error) {
	cur := strings.TrimSpace(cursor)
	if cur == "" {
		lookback := r.CursorLookback
		if lookback == 0 {
			lookback = defaultCursorLookback
		}
		if latest > lookback {
			return latest - lookback + 1, nil
		}
		return 0, nil
	}
	v, err := strconv.ParseUint(cur, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid cursor")
	}
	if v == ^uint64(0) {
		return v, nil
	}
	return v + 1, nil
}

func (r *RPCReader) blockNumber(ctx context.Context, ep endpoint.SelectedEndpoint) (uint64, error) {
	result := ""
	if err := r.call(ctx, ep, "eth_blockNumber", []any{}, &result); err != nil {
		return 0, err
	}
	return hexToUint64(result)
}

func (r *RPCReader) getBlockByNumber(ctx context.Context, ep endpoint.SelectedEndpoint, blockNum uint64) (rpcBlock, error) {
	var out rpcBlock
	err := r.call(ctx, ep, "eth_getBlockByNumber", []any{fmt.Sprintf("0x%x", blockNum), true}, &out)
	return out, err
}

func (r *RPCReader) getTransactionByHash(ctx context.Context, ep endpoint.SelectedEndpoint, txHash string) (rpcTx, bool, error) {
	var raw json.RawMessage
	if err := r.call(ctx, ep, "eth_getTransactionByHash", []any{txHash}, &raw); err != nil {
		return rpcTx{}, false, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return rpcTx{}, false, nil
	}
	var tx rpcTx
	if err := json.Unmarshal(raw, &tx); err != nil {
		return rpcTx{}, false, err
	}
	return tx, true, nil
}

func (r *RPCReader) getTransactionReceipt(ctx context.Context, ep endpoint.SelectedEndpoint, txHash string) (rpcReceipt, error) {
	var raw json.RawMessage
	if err := r.call(ctx, ep, "eth_getTransactionReceipt", []any{txHash}, &raw); err != nil {
		return rpcReceipt{}, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return rpcReceipt{}, nil
	}
	var receipt rpcReceipt
	if err := json.Unmarshal(raw, &receipt); err != nil {
		return rpcReceipt{}, err
	}
	return receipt, nil
}

func (r *RPCReader) call(ctx context.Context, ep endpoint.SelectedEndpoint, method string, params []any, out any) error {
	timeout := 10 * time.Second
	if ep.TimeoutMS > 0 {
		timeout = time.Duration(ep.TimeoutMS) * time.Millisecond
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	reqBody := rpcRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  method,
		Params:  params,
	}
	rawReq, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}
	httpClient := r.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: timeout}
	}
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, ep.URL, bytes.NewReader(rawReq))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("rpc status=%d method=%s", resp.StatusCode, method)
	}
	var rpcResp rpcResponse
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

func hexToUint64(v string) (uint64, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0, nil
	}
	if strings.HasPrefix(v, "0x") || strings.HasPrefix(v, "0X") {
		return strconv.ParseUint(v[2:], 16, 64)
	}
	return strconv.ParseUint(v, 10, 64)
}

func hexToBig(v string) (*big.Int, error) {
	x := strings.TrimSpace(v)
	if x == "" {
		return big.NewInt(0), nil
	}
	n := new(big.Int)
	var ok bool
	if strings.HasPrefix(x, "0x") || strings.HasPrefix(x, "0X") {
		_, ok = n.SetString(x[2:], 16)
	} else {
		_, ok = n.SetString(x, 10)
	}
	if !ok {
		return nil, fmt.Errorf("invalid number: %s", v)
	}
	return n, nil
}

func fallbackHash(v, fallback string) string {
	v = strings.TrimSpace(v)
	if v != "" {
		return v
	}
	return strings.TrimSpace(fallback)
}

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int64  `json:"id"`
	Method  string `json:"method"`
	Params  []any  `json:"params"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *rpcError       `json:"error"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcBlock struct {
	Number       string  `json:"number"`
	Transactions []rpcTx `json:"transactions"`
}

type rpcTx struct {
	Hash             string `json:"hash"`
	From             string `json:"from"`
	To               string `json:"to"`
	Value            string `json:"value"`
	TransactionIndex string `json:"transactionIndex"`
	BlockNumber      string `json:"blockNumber"`
}

type rpcReceipt struct {
	Status      string `json:"status"`
	BlockNumber string `json:"blockNumber"`
}
