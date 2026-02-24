module wallet-saas-v2/services/chain-gateway

go 1.26.0

require (
	github.com/dapplink-labs/wallet-chain-account v0.0.0
	github.com/dapplink-labs/wallet-chain-utxo v0.0.0
	google.golang.org/grpc v1.67.1
	google.golang.org/protobuf v1.35.1
)

require (
	golang.org/x/net v0.28.0 // indirect
	golang.org/x/sys v0.24.0 // indirect
	golang.org/x/text v0.17.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240814211410-ddb44dafa142 // indirect
)

replace github.com/dapplink-labs/wallet-chain-account => ../../legacy/wallet-chain-account

replace github.com/dapplink-labs/wallet-chain-utxo => ../../legacy/wallet-chain-utxo
