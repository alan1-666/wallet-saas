package evm

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"

	legacydispatcher "github.com/dapplink-labs/wallet-chain-account/chaindispatcher"
	legacy "github.com/dapplink-labs/wallet-chain-account/rpc/account"
	legacycommon "github.com/dapplink-labs/wallet-chain-account/rpc/common"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"wallet-saas-v2/services/chain-gateway/internal/clients"
	"wallet-saas-v2/services/chain-gateway/internal/ports"
)

type Adapter struct {
	Dispatcher *legacydispatcher.ChainDispatcher
}

func (a *Adapter) ConvertAddress(ctx context.Context, chain, addrType, publicKey string) (string, error) {
	resp, err := a.Dispatcher.ConvertAddress(ctx, &legacy.ConvertAddressRequest{Chain: chain, Network: "mainnet", Type: addrType, PublicKey: publicKey})
	if err != nil {
		return "", err
	}
	if resp.GetCode() == legacycommon.ReturnCode_ERROR {
		return "", clients.NewRPCError("account convert address", resp.GetMsg())
	}
	return resp.GetAddress(), nil
}

func (a *Adapter) SendTx(ctx context.Context, chain, network, coin, rawTx string) (string, error) {
	resp, err := a.Dispatcher.SendTx(ctx, &legacy.SendTxRequest{Chain: chain, Network: network, Coin: coin, RawTx: rawTx})
	if err != nil {
		return "", err
	}
	if resp.GetCode() == legacycommon.ReturnCode_ERROR {
		return "", clients.NewRPCError("account send tx", resp.GetMsg())
	}
	return resp.GetTxHash(), nil
}

func (a *Adapter) BuildUnsignedAccount(ctx context.Context, chain, network, base64Tx string) (string, error) {
	resp, err := a.Dispatcher.BuildUnSignTransaction(ctx, &legacy.UnSignTransactionRequest{Chain: chain, Network: network, Base64Tx: base64Tx})
	if err != nil {
		return "", err
	}
	if resp.GetCode() == legacycommon.ReturnCode_ERROR {
		return "", clients.NewRPCError("account build unsigned", resp.GetMsg())
	}
	return resp.GetUnSignTx(), nil
}

func (a *Adapter) BuildUnsignedUTXO(context.Context, string, string, string, []ports.TxVin, []ports.TxVout) (ports.BuildUnsignedResult, error) {
	return ports.BuildUnsignedResult{}, ports.ErrUnsupported
}

func (a *Adapter) BuildSignedUTXO(context.Context, string, string, []byte, [][]byte, [][]byte) ([]byte, string, error) {
	return nil, "", ports.ErrUnsupported
}

func (a *Adapter) SupportChains(ctx context.Context, chain, network string) (bool, error) {
	resp, err := a.Dispatcher.GetSupportChains(ctx, &legacy.SupportChainsRequest{Chain: chain, Network: network})
	if err != nil {
		return false, err
	}
	if resp.GetCode() == legacycommon.ReturnCode_ERROR {
		return false, clients.NewRPCError("account support chains", resp.GetMsg())
	}
	return resp.GetSupport(), nil
}

func (a *Adapter) ValidAddress(ctx context.Context, chain, network, _format, address string) (bool, error) {
	resp, err := a.Dispatcher.ValidAddress(ctx, &legacy.ValidAddressRequest{Chain: chain, Network: network, Address: address})
	if err != nil {
		return false, err
	}
	if resp.GetCode() == legacycommon.ReturnCode_ERROR {
		return false, clients.NewRPCError("account valid address", resp.GetMsg())
	}
	return resp.GetValid(), nil
}

func (a *Adapter) GetFee(ctx context.Context, in ports.FeeInput) (ports.FeeResult, error) {
	resp, err := a.Dispatcher.GetFee(ctx, &legacy.FeeRequest{Chain: in.Chain, Coin: in.Coin, Network: in.Network, RawTx: in.RawTx, Address: in.Address})
	if err != nil {
		return ports.FeeResult{}, err
	}
	if resp.GetCode() == legacycommon.ReturnCode_ERROR {
		return ports.FeeResult{}, clients.NewRPCError("account fee", resp.GetMsg())
	}
	return ports.FeeResult{SlowFee: resp.GetSlowFee(), NormalFee: resp.GetNormalFee(), FastFee: resp.GetFastFee()}, nil
}

func (a *Adapter) GetAccount(ctx context.Context, in ports.AccountInput) (ports.AccountResult, error) {
	resp, err := a.Dispatcher.GetAccount(ctx, &legacy.AccountRequest{Chain: in.Chain, Coin: in.Coin, Network: in.Network, Address: in.Address, ContractAddress: in.ContractAddress})
	if err != nil {
		return ports.AccountResult{}, err
	}
	if resp.GetCode() == legacycommon.ReturnCode_ERROR {
		return ports.AccountResult{}, clients.NewRPCError("account get account", resp.GetMsg())
	}
	return ports.AccountResult{Network: resp.GetNetwork(), AccountNumber: resp.GetAccountNumber(), Sequence: resp.GetSequence(), Balance: resp.GetBalance()}, nil
}

func (a *Adapter) GetTxByHash(ctx context.Context, in ports.TxQueryInput) (json.RawMessage, error) {
	resp, err := a.Dispatcher.GetTxByHash(ctx, &legacy.TxHashRequest{Chain: in.Chain, Coin: in.Coin, Network: in.Network, Hash: in.Hash})
	if err != nil {
		return nil, err
	}
	if resp.GetCode() == legacycommon.ReturnCode_ERROR {
		return nil, clients.NewRPCError("account tx by hash", resp.GetMsg())
	}
	return marshalProto(resp)
}

func (a *Adapter) GetTxByAddress(ctx context.Context, in ports.TxQueryInput) (json.RawMessage, error) {
	resp, err := a.Dispatcher.GetTxByAddress(ctx, &legacy.TxAddressRequest{
		Chain:           in.Chain,
		Coin:            in.Coin,
		Network:         in.Network,
		Address:         in.Address,
		ContractAddress: in.ContractAddress,
		Page:            in.Page,
		Pagesize:        in.PageSize,
	})
	if err != nil {
		return nil, err
	}
	if resp.GetCode() == legacycommon.ReturnCode_ERROR {
		return nil, clients.NewRPCError("account tx by address", resp.GetMsg())
	}
	return marshalProto(resp)
}

func (a *Adapter) GetUnspentOutputs(context.Context, string, string, string) (json.RawMessage, error) {
	return nil, ports.ErrUnsupported
}

func marshalProto(v proto.Message) (json.RawMessage, error) {
	b, err := protojson.Marshal(v)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func decodeHexOrBase64(s string) ([]byte, error) {
	if s == "" {
		return nil, nil
	}
	if b, err := hex.DecodeString(s); err == nil {
		return b, nil
	}
	return base64.StdEncoding.DecodeString(s)
}
