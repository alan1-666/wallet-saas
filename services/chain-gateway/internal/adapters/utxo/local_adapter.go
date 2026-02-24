package utxo

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"

	legacydispatcher "github.com/dapplink-labs/wallet-chain-utxo/chaindispatcher"
	legacycommon "github.com/dapplink-labs/wallet-chain-utxo/rpc/common"
	legacy "github.com/dapplink-labs/wallet-chain-utxo/rpc/utxo"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"wallet-saas-v2/services/chain-gateway/internal/clients"
	"wallet-saas-v2/services/chain-gateway/internal/ports"
)

type Adapter struct {
	Dispatcher *legacydispatcher.ChainDispatcher
}

func (a *Adapter) ConvertAddress(ctx context.Context, chain, addrType, publicKey string) (string, error) {
	resp, err := a.Dispatcher.ConvertAddress(ctx, &legacy.ConvertAddressRequest{Chain: chain, Network: "mainnet", Format: addrType, PublicKey: publicKey})
	if err != nil {
		return "", err
	}
	if resp.GetCode() == legacycommon.ReturnCode_ERROR {
		return "", clients.NewRPCError("utxo convert address", resp.GetMsg())
	}
	return resp.GetAddress(), nil
}

func (a *Adapter) SendTx(ctx context.Context, chain, network, coin, rawTx string) (string, error) {
	resp, err := a.Dispatcher.SendTx(ctx, &legacy.SendTxRequest{Chain: chain, Network: network, Coin: coin, RawTx: rawTx})
	if err != nil {
		return "", err
	}
	if resp.GetCode() == legacycommon.ReturnCode_ERROR {
		return "", clients.NewRPCError("utxo send tx", resp.GetMsg())
	}
	return resp.GetTxHash(), nil
}

func (a *Adapter) BuildUnsignedAccount(context.Context, string, string, string) (string, error) {
	return "", ports.ErrUnsupported
}

func (a *Adapter) BuildUnsignedUTXO(ctx context.Context, chain, network, fee string, vin []ports.TxVin, vout []ports.TxVout) (ports.BuildUnsignedResult, error) {
	pvIn := make([]*legacy.Vin, 0, len(vin))
	for _, x := range vin {
		pvIn = append(pvIn, &legacy.Vin{Hash: x.Hash, Index: x.Index, Amount: x.Amount, Address: x.Address})
	}
	pvOut := make([]*legacy.Vout, 0, len(vout))
	for _, x := range vout {
		pvOut = append(pvOut, &legacy.Vout{Address: x.Address, Amount: x.Amount, Index: x.Index})
	}
	resp, err := a.Dispatcher.CreateUnSignTransaction(ctx, &legacy.UnSignTransactionRequest{Chain: chain, Network: network, Fee: fee, Vin: pvIn, Vout: pvOut})
	if err != nil {
		return ports.BuildUnsignedResult{}, err
	}
	if resp.GetCode() == legacycommon.ReturnCode_ERROR {
		return ports.BuildUnsignedResult{}, clients.NewRPCError("utxo build unsigned", resp.GetMsg())
	}
	hashes := make([]string, 0, len(resp.GetSignHashes()))
	for _, h := range resp.GetSignHashes() {
		hashes = append(hashes, hex.EncodeToString(h))
	}
	return ports.BuildUnsignedResult{UnsignedTx: base64.StdEncoding.EncodeToString(resp.GetTxData()), SignHashes: hashes}, nil
}

func (a *Adapter) BuildSignedUTXO(ctx context.Context, chain, network string, txData []byte, signatures [][]byte, publicKeys [][]byte) ([]byte, string, error) {
	resp, err := a.Dispatcher.BuildSignedTransaction(ctx, &legacy.SignedTransactionRequest{
		Chain:      chain,
		Network:    network,
		TxData:     txData,
		Signatures: signatures,
		PublicKeys: publicKeys,
	})
	if err != nil {
		return nil, "", err
	}
	if resp.GetCode() == legacycommon.ReturnCode_ERROR {
		return nil, "", clients.NewRPCError("utxo build signed", resp.GetMsg())
	}
	return resp.GetSignedTxData(), hex.EncodeToString(resp.GetHash()), nil
}

func (a *Adapter) SupportChains(ctx context.Context, chain, network string) (bool, error) {
	resp, err := a.Dispatcher.GetSupportChains(ctx, &legacy.SupportChainsRequest{Chain: chain, Network: network})
	if err != nil {
		return false, err
	}
	if resp.GetCode() == legacycommon.ReturnCode_ERROR {
		return false, clients.NewRPCError("utxo support chains", resp.GetMsg())
	}
	return resp.GetSupport(), nil
}

func (a *Adapter) ValidAddress(ctx context.Context, chain, network, format, address string) (bool, error) {
	resp, err := a.Dispatcher.ValidAddress(ctx, &legacy.ValidAddressRequest{Chain: chain, Network: network, Format: format, Address: address})
	if err != nil {
		return false, err
	}
	if resp.GetCode() == legacycommon.ReturnCode_ERROR {
		return false, clients.NewRPCError("utxo valid address", resp.GetMsg())
	}
	return resp.GetValid(), nil
}

func (a *Adapter) GetFee(ctx context.Context, in ports.FeeInput) (ports.FeeResult, error) {
	resp, err := a.Dispatcher.GetFee(ctx, &legacy.FeeRequest{Chain: in.Chain, Coin: in.Coin, Network: in.Network, RawTx: in.RawTx})
	if err != nil {
		return ports.FeeResult{}, err
	}
	if resp.GetCode() == legacycommon.ReturnCode_ERROR {
		return ports.FeeResult{}, clients.NewRPCError("utxo fee", resp.GetMsg())
	}
	return ports.FeeResult{BestFee: resp.GetBestFee(), BestFeeSat: resp.GetBestFeeSat(), SlowFee: resp.GetSlowFee(), NormalFee: resp.GetNormalFee(), FastFee: resp.GetFastFee()}, nil
}

func (a *Adapter) GetAccount(ctx context.Context, in ports.AccountInput) (ports.AccountResult, error) {
	resp, err := a.Dispatcher.GetAccount(ctx, &legacy.AccountRequest{Chain: in.Chain, Network: in.Network, Address: in.Address, Brc20Address: in.ContractAddress})
	if err != nil {
		return ports.AccountResult{}, err
	}
	if resp.GetCode() == legacycommon.ReturnCode_ERROR {
		return ports.AccountResult{}, clients.NewRPCError("utxo get account", resp.GetMsg())
	}
	return ports.AccountResult{Network: resp.GetNetwork(), Balance: resp.GetBalance()}, nil
}

func (a *Adapter) GetTxByHash(ctx context.Context, in ports.TxQueryInput) (json.RawMessage, error) {
	resp, err := a.Dispatcher.GetTxByHash(ctx, &legacy.TxHashRequest{Chain: in.Chain, Coin: in.Coin, Network: in.Network, Hash: in.Hash})
	if err != nil {
		return nil, err
	}
	if resp.GetCode() == legacycommon.ReturnCode_ERROR {
		return nil, clients.NewRPCError("utxo tx by hash", resp.GetMsg())
	}
	return marshalProto(resp)
}

func (a *Adapter) GetTxByAddress(ctx context.Context, in ports.TxQueryInput) (json.RawMessage, error) {
	resp, err := a.Dispatcher.GetTxByAddress(ctx, &legacy.TxAddressRequest{Chain: in.Chain, Coin: in.Coin, Network: in.Network, Address: in.Address, Brc20Address: in.ContractAddress, Page: in.Page, Pagesize: in.PageSize, Cursor: in.Cursor})
	if err != nil {
		return nil, err
	}
	if resp.GetCode() == legacycommon.ReturnCode_ERROR {
		return nil, clients.NewRPCError("utxo tx by address", resp.GetMsg())
	}
	return marshalProto(resp)
}

func (a *Adapter) GetUnspentOutputs(ctx context.Context, chain, network, address string) (json.RawMessage, error) {
	resp, err := a.Dispatcher.GetUnspentOutputs(ctx, &legacy.UnspentOutputsRequest{Chain: chain, Network: network, Address: address})
	if err != nil {
		return nil, err
	}
	if resp.GetCode() == legacycommon.ReturnCode_ERROR {
		return nil, clients.NewRPCError("utxo unspent outputs", resp.GetMsg())
	}
	return marshalProto(resp)
}

func marshalProto(v proto.Message) (json.RawMessage, error) {
	b, err := protojson.Marshal(v)
	if err != nil {
		return nil, err
	}
	return b, nil
}
