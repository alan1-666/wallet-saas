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

type EthRPC struct {
	url  string
	http *http.Client
}

type EthTx struct {
	Hash   string
	From   string
	To     string
	Value  string
	Index  uint64
	Number uint64
}

func NewEthRPC(url string) *EthRPC {
	return &EthRPC{
		url: strings.TrimSpace(url),
		http: &http.Client{
			Timeout: 12 * time.Second,
		},
	}
}

func (e *EthRPC) Enabled() bool {
	return e != nil && e.url != ""
}

func (e *EthRPC) LatestBlockNumber(ctx context.Context) (uint64, error) {
	var out string
	if err := e.call(ctx, "eth_blockNumber", []any{}, &out); err != nil {
		return 0, err
	}
	return hexToUint64(out)
}

func (e *EthRPC) BlockTransactionsTo(ctx context.Context, fromBlock, toBlock uint64, toAddress string) ([]EthTx, uint64, error) {
	toAddr := strings.ToLower(strings.TrimSpace(toAddress))
	out := make([]EthTx, 0, 64)
	last := fromBlock
	for b := fromBlock; b <= toBlock; b++ {
		block, err := e.getBlockByNumber(ctx, b)
		if err != nil {
			return nil, last, err
		}
		last = b
		for _, tx := range block.Transactions {
			if strings.ToLower(strings.TrimSpace(tx.To)) != toAddr {
				continue
			}
			val, err := hexToUint64(tx.Value)
			if err != nil || val == 0 {
				continue
			}
			idx, _ := hexToUint64(tx.TransactionIndex)
			out = append(out, EthTx{
				Hash:   tx.Hash,
				From:   tx.From,
				To:     tx.To,
				Value:  strconv.FormatUint(val, 10),
				Index:  idx,
				Number: b,
			})
		}
	}
	return out, last, nil
}

type rpcReq struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
	Params  []any  `json:"params"`
}

type rpcResp struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *rpcErr         `json:"error"`
}

type rpcErr struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type blockByNumberResult struct {
	Number       string `json:"number"`
	Transactions []struct {
		Hash             string `json:"hash"`
		From             string `json:"from"`
		To               string `json:"to"`
		Value            string `json:"value"`
		TransactionIndex string `json:"transactionIndex"`
	} `json:"transactions"`
}

func (e *EthRPC) getBlockByNumber(ctx context.Context, blockNum uint64) (*blockByNumberResult, error) {
	var out blockByNumberResult
	if err := e.call(ctx, "eth_getBlockByNumber", []any{uint64ToHex(blockNum), true}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (e *EthRPC) call(ctx context.Context, method string, params []any, out any) error {
	reqBody := rpcReq{
		JSONRPC: "2.0",
		ID:      1,
		Method:  method,
		Params:  params,
	}
	raw, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.url, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := e.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("eth rpc status: %s", resp.Status)
	}
	var r rpcResp
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return err
	}
	if r.Error != nil {
		return fmt.Errorf("eth rpc error %d: %s", r.Error.Code, r.Error.Message)
	}
	if len(r.Result) == 0 || string(r.Result) == "null" {
		return fmt.Errorf("eth rpc empty result")
	}
	return json.Unmarshal(r.Result, out)
}

func hexToUint64(v string) (uint64, error) {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "0x")
	if v == "" {
		return 0, nil
	}
	n, err := strconv.ParseUint(v, 16, 64)
	if err != nil {
		return 0, err
	}
	return n, nil
}

func uint64ToHex(v uint64) string {
	return "0x" + strconv.FormatUint(v, 16)
}
