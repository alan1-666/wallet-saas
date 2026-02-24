package ports

import (
	"context"
	"encoding/json"
)

var ErrUnsupported = unsupportedError("unsupported operation")

type unsupportedError string

func (e unsupportedError) Error() string { return string(e) }

type TxVin struct {
	Hash    string
	Index   uint32
	Amount  int64
	Address string
}

type TxVout struct {
	Address string
	Amount  int64
	Index   uint32
}

type BuildUnsignedResult struct {
	UnsignedTx string
	SignHashes []string
}

type FeeInput struct {
	Chain   string
	Coin    string
	Network string
	RawTx   string
	Address string
}

type FeeResult struct {
	BestFee    string
	BestFeeSat string
	SlowFee    string
	NormalFee  string
	FastFee    string
}

type AccountInput struct {
	Chain           string
	Coin            string
	Network         string
	Address         string
	ContractAddress string
}

type AccountResult struct {
	Network       string
	AccountNumber string
	Sequence      string
	Balance       string
}

type TxQueryInput struct {
	Chain           string
	Coin            string
	Network         string
	Address         string
	ContractAddress string
	Hash            string
	Page            uint32
	PageSize        uint32
	Cursor          string
}

type ChainAdapter interface {
	ConvertAddress(ctx context.Context, chain, addrType, publicKey string) (string, error)
	SendTx(ctx context.Context, chain, network, coin, rawTx string) (string, error)
	BuildUnsignedAccount(ctx context.Context, chain, network, base64Tx string) (string, error)
	BuildUnsignedUTXO(ctx context.Context, chain, network, fee string, vin []TxVin, vout []TxVout) (BuildUnsignedResult, error)
	BuildSignedUTXO(ctx context.Context, chain, network string, txData []byte, signatures [][]byte, publicKeys [][]byte) ([]byte, string, error)
	SupportChains(ctx context.Context, chain, network string) (bool, error)
	ValidAddress(ctx context.Context, chain, network, format, address string) (bool, error)
	GetFee(ctx context.Context, in FeeInput) (FeeResult, error)
	GetAccount(ctx context.Context, in AccountInput) (AccountResult, error)
	GetTxByHash(ctx context.Context, in TxQueryInput) (json.RawMessage, error)
	GetTxByAddress(ctx context.Context, in TxQueryInput) (json.RawMessage, error)
	GetUnspentOutputs(ctx context.Context, chain, network, address string) (json.RawMessage, error)
}
