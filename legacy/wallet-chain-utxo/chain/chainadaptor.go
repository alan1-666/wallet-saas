package chain

import "github.com/dapplink-labs/wallet-chain-utxo/rpc/utxo"

type IChainAdaptor interface {
	GetSupportChains(req *utxo.SupportChainsRequest) (*utxo.SupportChainsResponse, error)
	ConvertAddress(req *utxo.ConvertAddressRequest) (*utxo.ConvertAddressResponse, error)
	ValidAddress(req *utxo.ValidAddressRequest) (*utxo.ValidAddressResponse, error)
	GetFee(req *utxo.FeeRequest) (*utxo.FeeResponse, error)
	GetAccount(req *utxo.AccountRequest) (*utxo.AccountResponse, error)
	GetUnspentOutputs(req *utxo.UnspentOutputsRequest) (*utxo.UnspentOutputsResponse, error)
	GetBlockByNumber(req *utxo.BlockNumberRequest) (*utxo.BlockResponse, error)
	GetBlockByHash(req *utxo.BlockHashRequest) (*utxo.BlockResponse, error)
	GetBlockHeaderByHash(req *utxo.BlockHeaderHashRequest) (*utxo.BlockHeaderResponse, error)
	GetBlockHeaderByNumber(req *utxo.BlockHeaderNumberRequest) (*utxo.BlockHeaderResponse, error)
	SendTx(req *utxo.SendTxRequest) (*utxo.SendTxResponse, error)
	GetTxByAddress(req *utxo.TxAddressRequest) (*utxo.TxAddressResponse, error)
	GetTxByHash(req *utxo.TxHashRequest) (*utxo.TxHashResponse, error)
	CreateUnSignTransaction(req *utxo.UnSignTransactionRequest) (*utxo.UnSignTransactionResponse, error)
	BuildSignedTransaction(req *utxo.SignedTransactionRequest) (*utxo.SignedTransactionResponse, error)
	DecodeTransaction(req *utxo.DecodeTransactionRequest) (*utxo.DecodeTransactionResponse, error)
	VerifySignedTransaction(req *utxo.VerifyTransactionRequest) (*utxo.VerifyTransactionResponse, error)
}
