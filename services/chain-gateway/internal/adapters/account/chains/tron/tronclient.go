package tron

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/fbsobreira/gotron-sdk/pkg/address"
	"github.com/go-resty/resty/v2"
)

const (
	defaultRequestTimeout = 10 * time.Second
	defaultRetryCount     = 3
	tronTRC20FeeLimit     = int64(100_000_000)
)

// TronClient Define a Tron RPC client
type TronClient struct {
	rpc *resty.Client
}

// DialTronClient Initialize and return a TronClient instance
func DialTronClient(rpcURL, rpcUser, rpcPass string) *TronClient {
	client := resty.New()
	client.SetHeader(rpcUser, rpcPass)
	client.SetBaseURL(rpcURL)
	client.SetTimeout(defaultRequestTimeout)
	client.SetRetryCount(defaultRetryCount)

	return &TronClient{
		rpc: client,
	}
}

// JsonRpc Call JSON-RPC
func (client *TronClient) JsonRpcBlock(params interface{}, result interface{}) error {
	// 构造请求体，确保 id_or_num 是字符串类型
	var idOrNum string
	switch v := params.(type) {
	case int64:
		idOrNum = fmt.Sprintf("\"%d\"", v) // 添加引号包裹数字
	case string:
		idOrNum = fmt.Sprintf("\"%s\"", v) // 添加引号包裹字符串
	default:
		return fmt.Errorf("unsupported params type: %T", params)
	}

	requestBody := map[string]interface{}{
		"id_or_num": json.RawMessage(idOrNum), // 使用 RawMessage 保持引号
		"detail":    true,
	}

	// 打印请求信息
	requestJSON, _ := json.MarshalIndent(requestBody, "", "  ")
	fmt.Printf("Request URL: %s\n", client.rpc.BaseURL+"/wallet/getblock")
	fmt.Printf("Request Headers:\n%v\n", client.rpc.Header)
	fmt.Printf("Request Body:\n%s\n", string(requestJSON))

	resp, err := client.rpc.R().
		SetBody(requestBody).
		SetResult(result).
		Post("/wallet/getblock")

	if err != nil {
		return fmt.Errorf("request failed: %v", err)
	}

	// 打印响应信息
	fmt.Printf("Response Status Code: %d\n", resp.StatusCode())
	fmt.Printf("Response Body:\n%s\n", string(resp.Body()))

	// 检查是否包含错误信息
	if strings.Contains(string(resp.Body()), "Error") {
		return fmt.Errorf("API error: %s", string(resp.Body()))
	}

	if resp.IsError() {
		return fmt.Errorf("API request failed with status code: %d", resp.StatusCode())
	}

	return nil
}

// JsonRpc Call JSON-RPC
func (client *TronClient) JsonRpcBlockHeader(params interface{}, result interface{}) error {
	// 构造请求体，确保 id_or_num 是字符串类型
	var idOrNum string
	switch v := params.(type) {
	case int64:
		idOrNum = fmt.Sprintf("\"%d\"", v) // 添加引号包裹数字
	case string:
		idOrNum = fmt.Sprintf("\"%s\"", v) // 添加引号包裹字符串
	default:
		return fmt.Errorf("unsupported params type: %T", params)
	}

	requestBody := map[string]interface{}{
		"id_or_num": json.RawMessage(idOrNum), // 使用 RawMessage 保持引号
		"detail":    false,
	}

	// 打印请求信息
	requestJSON, _ := json.MarshalIndent(requestBody, "", "  ")
	fmt.Printf("Request URL: %s\n", client.rpc.BaseURL+"/wallet/getblock")
	fmt.Printf("Request Headers:\n%v\n", client.rpc.Header)
	fmt.Printf("Request Body:\n%s\n", string(requestJSON))

	resp, err := client.rpc.R().
		SetBody(requestBody).
		SetResult(result).
		Post("/wallet/getblock")

	if err != nil {
		return fmt.Errorf("request failed: %v", err)
	}

	// 打印响应信息
	fmt.Printf("Response Status Code: %d\n", resp.StatusCode())
	fmt.Printf("Response Body:\n%s\n", string(resp.Body()))

	// 检查是否包含错误信息
	if strings.Contains(string(resp.Body()), "Error") {
		return fmt.Errorf("API error: %s", string(resp.Body()))
	}

	if resp.IsError() {
		return fmt.Errorf("API request failed with status code: %d", resp.StatusCode())
	}

	return nil
}

// Solidity Call Solidity
func (client *TronClient) Solidity(method string, params interface{}, result interface{}) error {
	_, err := client.rpc.R().SetBody(params).SetResult(result).Post("/walletsolidity/" + method)
	return err
}

// Wallet Call Wallet
func (client *TronClient) Wallet(method string, params interface{}, result interface{}) error {
	_, err := client.rpc.R().SetBody(params).SetResult(result).Post("/wallet/" + method)
	return err
}

// GetBlockByNumber Obtain block information based on block number
func (client *TronClient) GetBlockByNumber(blockNumber interface{}) (*BlockResponse, error) {

	var response BlockResponse
	err := client.JsonRpcBlock(blockNumber, &response)
	if err != nil {
		return nil, fmt.Errorf("failed to get block by number: %v", err)
	}
	return &response, nil
}

// GetBlockByNumber Obtain block information based on block number
func (client *TronClient) GetBlockByHush(hush string) (*Block, error) {
	params := []interface{}{hush, true}
	var response Response[Block]
	err := client.JsonRpcBlock(params, &response)
	if err != nil {
		return nil, fmt.Errorf("failed to get block by number: %v", err)
	}
	return &response.Result, nil
}

// GetBlockHeaderByNumber 获取区块头信息
func (client *TronClient) GetBlockHeaderByNumber(blockNumber int64) (*BlockResponse, error) {
	var response BlockResponse
	err := client.JsonRpcBlockHeader(blockNumber, &response)
	if err != nil {
		return nil, fmt.Errorf("failed to get block header: %v", err)
	}

	// 检查响应是否为空
	if response.BlockID == "" {
		return nil, fmt.Errorf("empty response received")
	}

	return &response, nil
}

// GetBlockByNumber Obtain block information based on block number
func (client *TronClient) GetBlockHeaderByHash(blockHush string) (*BlockResponse, error) {
	var response BlockResponse
	err := client.JsonRpcBlockHeader(blockHush, &response)
	if err != nil {
		return nil, fmt.Errorf("failed to get block header: %v", err)
	}

	// 检查响应是否为空
	if response.BlockID == "" {
		return nil, fmt.Errorf("empty response received")
	}

	return &response, nil
}

// GetBlockByHash Obtain block information based on block hash
func (client *TronClient) GetBlockByHash(blockHash string) (*Block, error) {
	params := []interface{}{blockHash, false}
	var response Response[Block]
	err := client.JsonRpcGetBlockByHash(params, &response)
	if err != nil {
		return nil, fmt.Errorf("failed to get block by hash: %v", err)

	}
	return &response.Result, nil
}
func (client *TronClient) JsonRpcGetBlockByHash(params interface{}, result interface{}) error {
	return nil
}
func (client *TronClient) JsonRpcGetBalance(params interface{}, result interface{}) error {

	requestBody := map[string]interface{}{
		"address": params, // 使用 RawMessage 保持引号
		"visible": true,
	}

	// 打印请求信息
	requestJSON, _ := json.MarshalIndent(requestBody, "", "  ")
	fmt.Printf("Request URL: %s\n", client.rpc.BaseURL+"/wallet/getblock")
	fmt.Printf("Request Headers:\n%v\n", client.rpc.Header)
	fmt.Printf("Request Body:\n%s\n", string(requestJSON))

	resp, err := client.rpc.R().
		SetBody(requestBody).
		SetResult(result).
		Post("/wallet/getaccount")

	if err != nil {
		return fmt.Errorf("request failed: %v", err)
	}

	// 打印响应信息
	fmt.Printf("Response Status Code: %d\n", resp.StatusCode())
	fmt.Printf("Response Body:\n%s\n", string(resp.Body()))

	// 检查是否包含错误信息
	if strings.Contains(string(resp.Body()), "Error") {
		return fmt.Errorf("API error: %s", string(resp.Body()))
	}

	if resp.IsError() {
		return fmt.Errorf("API request failed with status code: %d", resp.StatusCode())
	}

	return nil
}

// GetAccount Get account information
func (client *TronClient) GetBalance(address string) (*Account, error) {
	params := []interface{}{address}
	var response Account
	err := client.JsonRpcGetBalance(params, &response)
	if err != nil {
		return nil, fmt.Errorf("failed to get block by hash: %v", err)

	}
	return &response, nil

}

// GetAccount Get account information
func (client *TronClient) GetTransactionByHash(hush string) (*Transaction, error) {
	params := []interface{}{hush}
	var response Transaction
	err := client.JsonRpcGetTransactionByHash(params, &response)
	if err != nil {
		return nil, fmt.Errorf("failed to get block by hash: %v", err)

	}
	return &response, nil

}

func (client *TronClient) JsonRpcGetTransactionByHash(params interface{}, result interface{}) error {
	requestBody := map[string]interface{}{
		"value": params, // 使用 RawMessage 保持引号
	}

	// 打印请求信息
	requestJSON, _ := json.MarshalIndent(requestBody, "", "  ")
	fmt.Printf("Request URL: %s\n", client.rpc.BaseURL+"/wallet/getblock")
	fmt.Printf("Request Headers:\n%v\n", client.rpc.Header)
	fmt.Printf("Request Body:\n%s\n", string(requestJSON))

	resp, err := client.rpc.R().
		SetBody(requestBody).
		SetResult(result).
		Post("/walletsolidity/gettransactionbyid")

	if err != nil {
		return fmt.Errorf("request failed: %v", err)
	}

	// 打印响应信息
	fmt.Printf("Response Status Code: %d\n", resp.StatusCode())
	fmt.Printf("Response Body:\n%s\n", string(resp.Body()))

	// 检查是否包含错误信息
	if strings.Contains(string(resp.Body()), "Error") {
		return fmt.Errorf("API error: %s", string(resp.Body()))
	}

	if resp.IsError() {
		return fmt.Errorf("API request failed with status code: %d", resp.StatusCode())
	}

	return nil
}

func (client *TronClient) CreateTRXTransaction(fromAddress, toAddress string, amount int64) (*Transaction, error) {
	params := map[string]any{
		"owner_address": strings.TrimSpace(fromAddress),
		"to_address":    strings.TrimSpace(toAddress),
		"amount":        amount,
		"visible":       true,
	}
	var response Transaction
	if err := client.Wallet("createtransaction", params, &response); err != nil {
		return nil, err
	}
	if strings.TrimSpace(response.TxID) == "" {
		return nil, fmt.Errorf("empty tron unsigned transaction")
	}
	return &response, nil
}

func (client *TronClient) CreateTRC20Transaction(fromAddress, toAddress, contractAddress string, amount int64) (*Transaction, error) {
	to, err := address.Base58ToAddress(strings.TrimSpace(toAddress))
	if err != nil {
		return nil, fmt.Errorf("invalid tron to address: %w", err)
	}
	toBytes := to.Bytes()
	if len(toBytes) == 21 {
		toBytes = toBytes[1:]
	}
	if len(toBytes) != 20 {
		return nil, fmt.Errorf("invalid tron to address bytes length: %d", len(toBytes))
	}
	amountHex := new(big.Int).SetInt64(amount).Text(16)
	params := map[string]any{
		"owner_address":     strings.TrimSpace(fromAddress),
		"contract_address":  strings.TrimSpace(contractAddress),
		"function_selector": "transfer(address,uint256)",
		"parameter":         leftPadHex(hex.EncodeToString(toBytes), 64) + leftPadHex(amountHex, 64),
		"fee_limit":         tronTRC20FeeLimit,
		"call_value":        0,
		"visible":           true,
	}
	var response TriggerTransactionResponse
	if err := client.Wallet("triggersmartcontract", params, &response); err != nil {
		return nil, err
	}
	if !response.Result.Result {
		msg := strings.TrimSpace(response.Result.Message)
		if msg == "" {
			msg = "tron trigger smart contract failed"
		}
		return nil, fmt.Errorf("%s", msg)
	}
	if strings.TrimSpace(response.Transaction.TxID) == "" {
		return nil, fmt.Errorf("empty tron smart contract transaction")
	}
	return &response.Transaction, nil
}

func (client *TronClient) BroadcastTransaction(tx *Transaction) (*BroadcastReturns, error) {
	var response BroadcastReturns
	if err := client.Wallet("broadcasttransaction", tx, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func leftPadHex(value string, width int) string {
	value = strings.TrimSpace(strings.TrimPrefix(value, "0x"))
	if len(value) >= width {
		return value
	}
	return strings.Repeat("0", width-len(value)) + value
}
