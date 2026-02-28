package types

import "math/big"

type SpendingOutpointsItem struct {
	N       uint64  `json:"n"`
	TxIndex big.Int `json:"tx_index"`
}

type PrevOut struct {
	Addr              string                  `json:"addr"`
	N                 uint64                  `json:"n"`
	Script            string                  `json:"script"`
	SpendingOutpoints []SpendingOutpointsItem `json:"spending_outpoints"`
	Spent             bool                    `json:"spent"`
	TxIndex           big.Int                 `json:"tx_index"`
	Type              uint64                  `json:"type"`
	Value             big.Int                 `json:"value"`
}

type InputItem struct {
	Sequence big.Int `json:"sequence"`
	Witness  string  `json:"witness"`
	Script   string  `json:"script"`
	Index    uint64  `json:"index"`
	PrevOut  PrevOut `json:"prev_out"`
}

type OutItem struct {
	Type              uint64                  `json:"type"`
	Spent             bool                    `json:"spent"`
	Value             big.Int                 `json:"value"`
	SpendingOutpoints []SpendingOutpointsItem `json:"spending_outpoints"`
	N                 uint64                  `json:"n"`
	TxIndex           big.Int                 `json:"tx_index"`
	Script            string                  `json:"script"`
	Addr              string                  `json:"addr"`
}

type TxsItem struct {
	Hash        string      `json:"hash"`
	Ver         uint64      `json:"ver"`
	VinSz       uint64      `json:"vin_sz"`
	VoutSz      uint64      `json:"vout_sz"`
	Size        uint64      `json:"size"`
	Weight      uint64      `json:"weight"`
	Fee         big.Int     `json:"fee"`
	RelayedBy   string      `json:"relayed_by"`
	LockTime    big.Int     `json:"lock_time"`
	TxIndex     uint64      `json:"tx_index"`
	DoubleSpend bool        `json:"double_spend"`
	Time        big.Int     `json:"time"`
	BlockIndex  big.Int     `json:"block_index"`
	BlockHeight big.Int     `json:"block_height"`
	Inputs      []InputItem `json:"inputs"`
	Out         []OutItem   `json:"out"`
	Result      big.Int     `json:"result"`
	Balance     big.Int     `json:"balance"`
}

type Transaction struct {
	Hash160       string    `json:"hash160"`
	Address       string    `json:"address"`
	NTx           uint64    `json:"n_tx"`
	NUnredeemed   big.Int   `json:"n_unredeemed"`
	TotalReceived big.Int   `json:"total_received"`
	TotalSent     big.Int   `json:"total_sent"`
	FinalBalance  big.Int   `json:"final_balance"`
	Txs           []TxsItem `json:"txs"`
}

type BlockData struct {
	Hash              string   `json:"hash"`
	Confirmations     uint64   `json:"confirmations"`
	Size              uint64   `json:"size"`
	StrippedSize      uint64   `json:"strippedsize"`
	Weight            uint64   `json:"weight"`
	Height            uint64   `json:"height"`
	Version           uint64   `json:"version"`
	VersionHex        string   `json:"version_hex"`
	Merkleroot        string   `json:"merkleroot"`
	Tx                []string `json:"tx"`
	Time              uint64   `json:"time"`
	MedianTime        uint64   `json:"mediantime"`
	Nonce             uint64   `json:"nonce"`
	Bits              string   `json:"bits"`
	Difficulty        uint64   `json:"difficulty"`
	ChainWork         string   `json:"chainwork"`
	NTx               uint64   `json:"nTx"`
	PreviousBlockHash string   `json:"previousblockhash"`
	NextBlockHash     string   `json:"nextblockhash"`
}

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
	Value        uint64       `json:"value"`
	N            uint64       `json:"n"`
	ScriptPubKey ScriptPubKey `json:"scriptpubkey"`
}

type RawTransactionData struct {
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

type AccountBalance struct {
	FinalBalance  big.Int `json:"final_balance"`
	NTx           big.Int `json:"n_tx"`
	TotalReceived big.Int `json:"total_received"`
}

type UnspentOutput struct {
	TxHashBigEndian string `json:"tx_hash_big_endian"`
	TxHash          string `json:"tx_hash"`
	TxOutputN       uint64 `json:"tx_output_n"`
	Script          string `json:"script"`
	Value           uint64 `json:"value"`
	ValueHex        string `json:"value_hex"`
	Confirmations   uint64 `json:"confirmations"`
	TxIndex         uint64 `json:"tx_index"`
}

type UnspentOutputList struct {
	Notice         string          `json:"notice"`
	UnspentOutputs []UnspentOutput `json:"unspent_outputs"`
}

type GasFee struct {
	ChainFullName       string `json:"chainFullName"`
	ChainShortName      string `json:"chainShortName"`
	Symbol              string `json:"symbol"`
	BestTransactionFee  string `json:"bestTransactionFee"`
	RecommendedGasPrice string `json:"recommendedGasPrice"`
	RapidGasPrice       string `json:"rapidGasPrice"`
	StandardGasPrice    string `json:"standardGasPrice"`
	SlowGasPrice        string `json:"slowGasPrice"`
}

type GasFeeData struct {
	Code string   `json:"code"`
	Msg  string   `json:"msg"`
	Data []GasFee `json:"data"`
}
