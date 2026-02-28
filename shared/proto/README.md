# shared/proto

Put stable cross-service protobuf contracts here.

Initial migration targets:
- wallet-sign-go/protobuf/wallet.proto
- multichain-sync-account/protobuf/dapplink-wallet.proto
- wallet-chain-account/rpc/account/*.proto
- wallet-chain-utxo/rpc/utxo/*.proto

## Regenerate `chaingateway.proto`

Run from repo root:

```bash
ROOT="$(git rev-parse --show-toplevel)"
mkdir -p services/scan-service/internal/pb/chaingateway
cd shared/proto
PATH="$PATH:$HOME/go/bin" protoc \
  --go_out=paths=source_relative,Mchaingateway.proto=wallet-saas-v2/shared/proto/chaingateway\;chaingateway:$ROOT/services/chain-gateway/internal/pb/chaingateway \
  --go-grpc_out=paths=source_relative,Mchaingateway.proto=wallet-saas-v2/shared/proto/chaingateway\;chaingateway:$ROOT/services/chain-gateway/internal/pb/chaingateway \
  chaingateway.proto
PATH="$PATH:$HOME/go/bin" protoc \
  --go_out=paths=source_relative,Mchaingateway.proto=wallet-saas-v2/shared/proto/chaingateway\;chaingateway:$ROOT/services/wallet-core/internal/pb/chaingateway \
  --go-grpc_out=paths=source_relative,Mchaingateway.proto=wallet-saas-v2/shared/proto/chaingateway\;chaingateway:$ROOT/services/wallet-core/internal/pb/chaingateway \
  chaingateway.proto
PATH="$PATH:$HOME/go/bin" protoc \
  --go_out=paths=source_relative,Mchaingateway.proto=wallet-saas-v2/shared/proto/chaingateway\;chaingateway:$ROOT/services/scan-service/internal/pb/chaingateway \
  --go-grpc_out=paths=source_relative,Mchaingateway.proto=wallet-saas-v2/shared/proto/chaingateway\;chaingateway:$ROOT/services/scan-service/internal/pb/chaingateway \
  chaingateway.proto
```
