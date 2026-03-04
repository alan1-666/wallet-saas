package evm

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	gethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"

	accountadapter "wallet-saas-v2/services/chain-gateway/internal/adapters/account"
	"wallet-saas-v2/services/chain-gateway/internal/clients"
	"wallet-saas-v2/services/chain-gateway/internal/endpoint"
	"wallet-saas-v2/services/chain-gateway/internal/ports"
)

func New(chain string, reader *RPCReader) accountadapter.ChainPlugin {
	return &Plugin{
		ChainName: clients.NormalizeChain(chain),
		Reader:    reader,
	}
}

type Plugin struct {
	accountadapter.BasePlugin
	ChainName string
	Reader    *RPCReader
}

func (p *Plugin) Chain() string {
	if strings.TrimSpace(p.ChainName) == "" {
		return "ethereum"
	}
	return p.ChainName
}

func (p *Plugin) SupportChains(_ context.Context, chain, _ string) (bool, error) {
	return clients.IsEVMChain(chain), nil
}

func (p *Plugin) ConvertAddress(_ context.Context, chain, _network, _addrType, publicKey string) (string, error) {
	if !clients.IsEVMChain(chain) {
		return "", fmt.Errorf("unsupported chain: %s", chain)
	}
	pub, err := parseECDSAPublicKey(publicKey)
	if err != nil {
		return "", err
	}
	return gethcrypto.PubkeyToAddress(*pub).Hex(), nil
}

func (p *Plugin) ValidAddress(_ context.Context, chain, _network, _format, address string) (bool, error) {
	if !clients.IsEVMChain(chain) {
		return false, nil
	}
	return gethcommon.IsHexAddress(strings.TrimSpace(address)), nil
}

func (p *Plugin) BuildUnsignedAccount(_ context.Context, chain, _network, base64Tx string) (ports.BuildUnsignedResult, error) {
	if !clients.IsEVMChain(chain) {
		return ports.BuildUnsignedResult{}, fmt.Errorf("unsupported chain: %s", chain)
	}
	payload := strings.TrimSpace(base64Tx)
	raw, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return ports.BuildUnsignedResult{}, fmt.Errorf("invalid base64_tx: %w", err)
	}
	dynamicTx, err := decodeDynamicFeeTx(raw)
	if err != nil {
		return ports.BuildUnsignedResult{}, err
	}
	tx := types.NewTx(dynamicTx)
	signer := types.LatestSignerForChainID(dynamicTx.ChainID)
	hash := signer.Hash(tx)
	return ports.BuildUnsignedResult{
		UnsignedTx: payload,
		SignHashes: []string{hash.Hex()},
	}, nil
}

func (p *Plugin) BuildSignedAccount(_ context.Context, chain, _network, base64Tx, signature, publicKey string) (string, error) {
	if !clients.IsEVMChain(chain) {
		return "", fmt.Errorf("unsupported chain: %s", chain)
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(base64Tx))
	if err != nil {
		return "", fmt.Errorf("invalid base64_tx: %w", err)
	}
	dynamicTx, err := decodeDynamicFeeTx(raw)
	if err != nil {
		return "", err
	}
	tx := types.NewTx(dynamicTx)
	signer := types.LatestSignerForChainID(dynamicTx.ChainID)
	sigBytes, err := decodeHexOrBase64Sig(signature)
	if err != nil {
		return "", fmt.Errorf("invalid signature: %w", err)
	}
	sig65, err := normalizeEVMSignature(tx, signer, sigBytes, publicKey)
	if err != nil {
		return "", err
	}
	signedTx, err := tx.WithSignature(signer, sig65)
	if err != nil {
		return "", fmt.Errorf("attach signature failed: %w", err)
	}
	encoded, err := signedTx.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("marshal signed tx failed: %w", err)
	}
	return "0x" + hex.EncodeToString(encoded), nil
}

func (p *Plugin) SendTx(ctx context.Context, chain, network, _coin, rawTx string) (string, error) {
	if !clients.IsEVMChain(chain) {
		return "", fmt.Errorf("unsupported chain: %s", chain)
	}
	ep, err := p.selectEndpoint(chain, network)
	if err != nil {
		return "", err
	}
	out := ""
	if err := p.callRPC(ctx, ep, "eth_sendRawTransaction", []any{strings.TrimSpace(rawTx)}, &out); err != nil {
		p.Reader.ReportFailure(ep.Key, err)
		return "", err
	}
	p.Reader.ReportSuccess(ep.Key)
	return strings.TrimSpace(out), nil
}

func (p *Plugin) GetAccount(ctx context.Context, in ports.AccountInput) (ports.AccountResult, error) {
	if p.Reader == nil {
		return ports.AccountResult{}, fmt.Errorf("evm reader is nil")
	}
	out, err := p.Reader.GetBalance(ctx, in.Chain, in.Network, in.Address)
	if err != nil {
		return ports.AccountResult{}, err
	}
	return ports.AccountResult{
		Network:       out.Network,
		AccountNumber: "0",
		Sequence:      out.Sequence,
		Balance:       out.Balance,
	}, nil
}

func (p *Plugin) GetTxByHash(ctx context.Context, in ports.TxQueryInput) (json.RawMessage, error) {
	if p.Reader == nil {
		return nil, fmt.Errorf("evm reader is nil")
	}
	out, err := p.Reader.GetTxFinality(ctx, in.Chain, in.Network, in.Hash)
	if err != nil {
		return nil, err
	}
	payload := map[string]any{
		"tx_hash":       out.TxHash,
		"confirmations": out.Confirmations,
		"status":        out.Status,
		"found":         out.Found,
	}
	return json.Marshal(payload)
}

func (p *Plugin) GetTxByAddress(ctx context.Context, in ports.TxQueryInput) (json.RawMessage, error) {
	if p.Reader == nil {
		return nil, fmt.Errorf("evm reader is nil")
	}
	out, err := p.Reader.ListIncomingTransfers(ctx, in.Chain, in.Network, in.Address, in.Cursor, in.PageSize)
	if err != nil {
		return nil, err
	}
	items := make([]map[string]any, 0, len(out.Items))
	for idx, item := range out.Items {
		items = append(items, map[string]any{
			"hash":          item.TxHash,
			"index":         fallbackIndex(item.Index, idx),
			"froms":         []map[string]any{{"address": item.FromAddress}},
			"tos":           []map[string]any{{"address": item.ToAddress}},
			"values":        []map[string]any{{"value": item.Amount}},
			"confirmations": item.Confirmations,
			"status":        item.Status,
		})
	}
	payload := map[string]any{
		"tx":          items,
		"next_cursor": out.NextCursor,
	}
	return json.Marshal(payload)
}

func parseECDSAPublicKey(raw string) (*ecdsa.PublicKey, error) {
	v := strings.TrimSpace(raw)
	v = strings.TrimPrefix(v, "0x")
	if v == "" {
		return nil, fmt.Errorf("public key is required")
	}
	b, err := hex.DecodeString(v)
	if err != nil {
		return nil, fmt.Errorf("invalid public key: %w", err)
	}
	switch len(b) {
	case 33:
		pub, err := gethcrypto.DecompressPubkey(b)
		if err != nil {
			return nil, fmt.Errorf("invalid compressed public key: %w", err)
		}
		return pub, nil
	case 64:
		pub, err := gethcrypto.UnmarshalPubkey(append([]byte{0x04}, b...))
		if err != nil {
			return nil, fmt.Errorf("invalid public key: %w", err)
		}
		return pub, nil
	case 65:
		if b[0] != 0x04 {
			return nil, fmt.Errorf("invalid uncompressed public key prefix")
		}
		pub, err := gethcrypto.UnmarshalPubkey(b)
		if err != nil {
			return nil, fmt.Errorf("invalid public key: %w", err)
		}
		return pub, nil
	default:
		return nil, fmt.Errorf("invalid public key length=%d", len(b))
	}
}

func fallbackIndex(v int64, idx int) int64 {
	if v > 0 {
		return v
	}
	return int64(idx)
}

func decodeHexOrBase64Sig(raw string) ([]byte, error) {
	v := strings.TrimSpace(raw)
	v = strings.TrimPrefix(v, "0x")
	if v == "" {
		return nil, fmt.Errorf("signature is required")
	}
	if b, err := hex.DecodeString(v); err == nil {
		return b, nil
	}
	return base64.StdEncoding.DecodeString(v)
}

func normalizeEVMSignature(tx *types.Transaction, signer types.Signer, sig []byte, publicKey string) ([]byte, error) {
	switch len(sig) {
	case 64:
		base := make([]byte, 65)
		copy(base, sig)
		if strings.TrimSpace(publicKey) == "" {
			base[64] = 0
			return base, nil
		}
		for _, v := range []byte{0, 1} {
			trial := make([]byte, len(base))
			copy(trial, base)
			trial[64] = v
			if matchesSignerPublicKey(tx, signer, trial, publicKey) {
				return trial, nil
			}
		}
		return nil, fmt.Errorf("unable to determine recovery id from signature/public_key")
	case 65:
		out := make([]byte, 65)
		copy(out, sig)
		if out[64] >= 27 {
			out[64] -= 27
		}
		if out[64] > 1 {
			return nil, fmt.Errorf("invalid signature recovery id=%d", out[64])
		}
		if strings.TrimSpace(publicKey) == "" || matchesSignerPublicKey(tx, signer, out, publicKey) {
			return out, nil
		}
		flip := make([]byte, 65)
		copy(flip, out)
		flip[64] = 1 - flip[64]
		if matchesSignerPublicKey(tx, signer, flip, publicKey) {
			return flip, nil
		}
		return nil, fmt.Errorf("signature does not match public_key")
	default:
		return nil, fmt.Errorf("invalid signature length=%d", len(sig))
	}
}

func matchesSignerPublicKey(tx *types.Transaction, signer types.Signer, sig []byte, publicKey string) bool {
	signedTx, err := tx.WithSignature(signer, sig)
	if err != nil {
		return false
	}
	sender, err := types.Sender(signer, signedTx)
	if err != nil {
		return false
	}
	pub, err := parseECDSAPublicKey(publicKey)
	if err != nil {
		return false
	}
	expected := gethcrypto.PubkeyToAddress(*pub)
	return strings.EqualFold(sender.Hex(), expected.Hex())
}

type evmDynamicFeeRequest struct {
	ChainID              string `json:"chainId"`
	Nonce                uint64 `json:"nonce"`
	MaxPriorityFeePerGas string `json:"maxPriorityFeePerGas"`
	MaxFeePerGas         string `json:"maxFeePerGas"`
	GasLimit             uint64 `json:"gasLimit"`
	ToAddress            string `json:"toAddress"`
	Amount               string `json:"amount"`
	ContractAddress      string `json:"contractAddress"`
}

func decodeDynamicFeeTx(raw []byte) (*types.DynamicFeeTx, error) {
	var req evmDynamicFeeRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, fmt.Errorf("invalid dynamic fee tx json: %w", err)
	}
	chainID, ok := new(big.Int).SetString(strings.TrimSpace(req.ChainID), 10)
	if !ok {
		return nil, fmt.Errorf("invalid chainId")
	}
	maxPriorityFee, ok := new(big.Int).SetString(strings.TrimSpace(req.MaxPriorityFeePerGas), 10)
	if !ok {
		return nil, fmt.Errorf("invalid maxPriorityFeePerGas")
	}
	maxFee, ok := new(big.Int).SetString(strings.TrimSpace(req.MaxFeePerGas), 10)
	if !ok {
		return nil, fmt.Errorf("invalid maxFeePerGas")
	}
	amount, ok := new(big.Int).SetString(strings.TrimSpace(req.Amount), 10)
	if !ok {
		return nil, fmt.Errorf("invalid amount")
	}
	toAddress := gethcommon.HexToAddress(strings.TrimSpace(req.ToAddress))
	contractAddress := strings.ToLower(strings.TrimSpace(req.ContractAddress))

	var txTo gethcommon.Address
	var txValue *big.Int
	var txData []byte
	if contractAddress == "" || contractAddress == "0x00" || contractAddress == "0x0000000000000000000000000000000000000000" {
		txTo = toAddress
		txValue = amount
	} else {
		txTo = gethcommon.HexToAddress(req.ContractAddress)
		txValue = big.NewInt(0)
		txData = buildERC20TransferData(toAddress, amount)
	}
	return &types.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     req.Nonce,
		GasTipCap: maxPriorityFee,
		GasFeeCap: maxFee,
		Gas:       req.GasLimit,
		To:        &txTo,
		Value:     txValue,
		Data:      txData,
	}, nil
}

func buildERC20TransferData(toAddress gethcommon.Address, amount *big.Int) []byte {
	methodID := gethcrypto.Keccak256([]byte("transfer(address,uint256)"))[:4]
	addr := gethcommon.LeftPadBytes(toAddress.Bytes(), 32)
	amt := gethcommon.LeftPadBytes(amount.Bytes(), 32)
	data := make([]byte, 0, 4+32+32)
	data = append(data, methodID...)
	data = append(data, addr...)
	data = append(data, amt...)
	return data
}

func (p *Plugin) selectEndpoint(chain, network string) (endpoint.SelectedEndpoint, error) {
	if p == nil || p.Reader == nil {
		return endpoint.SelectedEndpoint{}, fmt.Errorf("no rpc endpoint configured")
	}
	return p.Reader.SelectEndpoint(chain, network)
}

func (p *Plugin) callRPC(ctx context.Context, ep endpoint.SelectedEndpoint, method string, params []any, out any) error {
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
	client := &http.Client{Timeout: timeout}
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
	var rpcResp struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return err
	}
	if rpcResp.Error != nil {
		return fmt.Errorf("rpc method=%s code=%d msg=%s", method, rpcResp.Error.Code, rpcResp.Error.Message)
	}
	if out == nil || len(rpcResp.Result) == 0 {
		return nil
	}
	return json.Unmarshal(rpcResp.Result, out)
}
