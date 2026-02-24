package base

type ScriptSig struct {
	Asm string `json:"asm"`
	Hex string `json:"hex"`
}

type Vin struct {
	TxId        string    `json:"txid"`
	Vout        uint64    `json:"vout"`
	ScriptSig   ScriptSig `json:"scriptSig"`
	Sequence    uint64    `json:"sequence"`
	TxInWitness []string  `json:"txinwitness"`
}

type ScriptPubKey struct {
	Asm     string `json:"asm"`
	Hex     string `json:"hex"`
	Desc    string `json:"desc"`
	Address string `json:"addresses"`
	Type    string `json:"type"`
}

type Vout struct {
	Value        interface{}  `json:"value"`
	N            uint64       `json:"n"`
	ScriptPubKey ScriptPubKey `json:"scriptpubkey"`
}

type ResultRawData struct {
	TxId          string `json:"txid"`
	Hash          string `json:"hash"`
	Version       uint64 `json:"version"`
	Size          uint64 `json:"size"`
	VSize         uint64 `json:"vsize"`
	Weight        uint64 `json:"weight"`
	LockTime      uint64 `json:"locktime"`
	Vin           []Vin  `json:"vin"`
	Vout          []Vout `json:"vout"`
	Hex           string `json:"hex"`
	Blockhash     string `json:"blockhash"`
	Confirmations uint64 `json:"confirmations"`
	BlockTime     uint64 `json:"blocktime"`
	Time          uint64 `json:"time"`
}
