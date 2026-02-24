package litecoin

import (
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/wire"
)

type CustomParamStruct struct {
	Net              wire.BitcoinNet
	PubKeyHashAddrID byte
	ScriptHashAddrID byte
	Bech32HRPSegwit  string
}

var CustomParams = CustomParamStruct{
	Net:              0xdbb6c0fb,
	PubKeyHashAddrID: 0x30,
	ScriptHashAddrID: 0x32,
	Bech32HRPSegwit:  "ltc",
}

func applyCustomParams(params chaincfg.Params, customParams CustomParamStruct) chaincfg.Params {
	params.Net = customParams.Net
	params.PubKeyHashAddrID = customParams.PubKeyHashAddrID
	params.ScriptHashAddrID = customParams.ScriptHashAddrID
	params.Bech32HRPSegwit = customParams.Bech32HRPSegwit
	return params
}

var customParams = applyCustomParams(chaincfg.MainNetParams, CustomParams)
