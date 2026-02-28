package ports

type ChainModel string

const (
	ModelAccount ChainModel = "account"
	ModelUTXO    ChainModel = "utxo"
)

type PluginBinding struct {
	Chain   string
	Network string
	Model   ChainModel
	Adapter ChainAdapter
}

type IncomingTransferInput struct {
	Model    ChainModel
	Chain    string
	Coin     string
	Network  string
	Address  string
	Cursor   string
	Page     uint32
	PageSize uint32
}

type IncomingTransfer struct {
	TxHash        string `json:"tx_hash"`
	FromAddress   string `json:"from_address"`
	ToAddress     string `json:"to_address"`
	Amount        string `json:"amount"`
	Confirmations int64  `json:"confirmations"`
	Index         int64  `json:"index"`
	Status        string `json:"status"`
}

type IncomingTransferResult struct {
	Items      []IncomingTransfer `json:"items"`
	NextCursor string             `json:"next_cursor,omitempty"`
}

type TxFinalityInput struct {
	Chain   string
	Coin    string
	Network string
	TxHash  string
}

type TxFinality struct {
	TxHash        string `json:"tx_hash"`
	Confirmations int64  `json:"confirmations"`
	Status        string `json:"status"`
	Found         bool   `json:"found"`
}

type BalanceInput struct {
	Chain           string
	Coin            string
	Network         string
	Address         string
	ContractAddress string
}

type BalanceResult struct {
	Balance  string `json:"balance"`
	Network  string `json:"network,omitempty"`
	Sequence string `json:"sequence,omitempty"`
}
