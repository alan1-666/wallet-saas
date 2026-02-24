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

func IsUTXOChain(chain string) bool {
	_, ok := utxoChains[strings.ToLower(strings.TrimSpace(chain))]
	return ok
}
