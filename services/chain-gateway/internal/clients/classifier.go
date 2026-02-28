package clients

import "strings"

var utxoChains = map[string]struct{}{
	"bitcoin": {}, "btc": {},
	"bitcoincash": {}, "bch": {},
	"dash":     {},
	"dogecoin": {}, "doge": {},
	"litecoin": {}, "ltc": {},
	"zen": {},
}

var evmChains = map[string]struct{}{
	"ethereum": {},
	"binance":  {},
	"polygon":  {},
	"arbitrum": {},
	"optimism": {},
	"linea":    {},
	"scroll":   {},
	"mantle":   {},
	"zksync":   {},
}

var chainAlias = map[string]string{
	"eth":  "ethereum",
	"bsc":  "binance",
	"trx":  "tron",
	"sol":  "solana",
	"btc":  "bitcoin",
	"bch":  "bitcoincash",
	"ltc":  "litecoin",
	"doge": "dogecoin",
}

func NormalizeChain(chain string) string {
	v := strings.ToLower(strings.TrimSpace(chain))
	if v == "" {
		return ""
	}
	if canonical, ok := chainAlias[v]; ok {
		return canonical
	}
	return v
}

func IsUTXOChain(chain string) bool {
	_, ok := utxoChains[NormalizeChain(chain)]
	return ok
}

func IsEVMChain(chain string) bool {
	_, ok := evmChains[NormalizeChain(chain)]
	return ok
}
