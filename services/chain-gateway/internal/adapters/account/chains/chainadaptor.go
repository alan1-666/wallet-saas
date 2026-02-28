package chain

import "wallet-saas-v2/services/chain-gateway/internal/adapters/account/chains/rpc/account"

type IChainAdaptor interface {
	GetSupportChains(req *account.SupportChainsRequest) (*account.SupportChainsResponse, error)
	ConvertAddress(req *account.ConvertAddressRequest) (*account.ConvertAddressResponse, error)
	ValidAddress(req *account.ValidAddressRequest) (*account.ValidAddressResponse, error)
	GetBlockByNumber(req *account.BlockNumberRequest) (*account.BlockResponse, error)
	GetBlockByHash(req *account.BlockHashRequest) (*account.BlockResponse, error)
	GetBlockHeaderByHash(req *account.BlockHeaderHashRequest) (*account.BlockHeaderResponse, error)
	GetBlockHeaderByNumber(req *account.BlockHeaderNumberRequest) (*account.BlockHeaderResponse, error)
	GetAccount(req *account.AccountRequest) (*account.AccountResponse, error)
	GetFee(req *account.FeeRequest) (*account.FeeResponse, error)
	SendTx(req *account.SendTxRequest) (*account.SendTxResponse, error)
	GetTxByAddress(req *account.TxAddressRequest) (*account.TxAddressResponse, error)
	GetTxByHash(req *account.TxHashRequest) (*account.TxHashResponse, error)
	GetBlockByRange(req *account.BlockByRangeRequest) (*account.BlockByRangeResponse, error)
	BuildUnSignTransaction(req *account.UnSignTransactionRequest) (*account.UnSignTransactionResponse, error)
	BuildSignedTransaction(req *account.SignedTransactionRequest) (*account.SignedTransactionResponse, error)
	DecodeTransaction(req *account.DecodeTransactionRequest) (*account.DecodeTransactionResponse, error)
	VerifySignedTransaction(req *account.VerifyTransactionRequest) (*account.VerifyTransactionResponse, error)
	GetExtraData(req *account.ExtraDataRequest) (*account.ExtraDataResponse, error)
	GetNftListByAddress(req *account.NftAddressRequest) (*account.NftAddressResponse, error)
}
