#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://127.0.0.1:8081}"
TENANT_ID="${TENANT_ID:-t1}"
USER_ACCOUNT_ID="${USER_ACCOUNT_ID:-u1001}"
TREASURY_ACCOUNT_ID="${TREASURY_ACCOUNT_ID:-treasury-main}"

REQ() {
  local method="$1"; shift
  local path="$1"; shift
  local body="${1:-}"
  if [[ -n "$body" ]]; then
    curl -sS -X "$method" "$BASE_URL$path" -H 'Content-Type: application/json' -d "$body"
  else
    curl -sS -X "$method" "$BASE_URL$path"
  fi
}

log() { echo "[e2e] $*"; }

ensure_account() {
  local account_id="$1"
  REQ POST /v1/account/upsert "{\"tenant_id\":\"$TENANT_ID\",\"account_id\":\"$account_id\",\"status\":\"ACTIVE\"}" >/dev/null
}

create_addr() {
  local chain="$1" network="$2" coin="$3" sign_type="$4"
  REQ POST /v1/address/create "{\"tenant_id\":\"$TENANT_ID\",\"account_id\":\"$USER_ACCOUNT_ID\",\"chain\":\"$chain\",\"coin\":\"$coin\",\"network\":\"$network\",\"sign_type\":\"$sign_type\",\"model\":\"account\"}"
}

create_addr_treasury() {
  local chain="$1" network="$2" coin="$3" sign_type="$4"
  REQ POST /v1/address/create "{\"tenant_id\":\"$TENANT_ID\",\"account_id\":\"$TREASURY_ACCOUNT_ID\",\"chain\":\"$chain\",\"coin\":\"$coin\",\"network\":\"$network\",\"sign_type\":\"$sign_type\",\"model\":\"account\"}"
}

extract_field() {
  local field="$1"
  python3 - "$field" <<'PY'
import json,sys
field=sys.argv[1]
obj=json.load(sys.stdin)
print(obj.get(field,""))
PY
}

log "health check"
REQ GET /healthz >/dev/null

log "upsert accounts"
ensure_account "$USER_ACCOUNT_ID"
ensure_account "$TREASURY_ACCOUNT_ID"

# chain network coin sign_type
CHAINS=(
  "ethereum sepolia ETH ecdsa"
  "binance testnet BNB ecdsa"
  "polygon amoy MATIC ecdsa"
  "arbitrum sepolia ETH ecdsa"
  "solana devnet SOL eddsa"
  "tron nile TRX ecdsa"
)

for item in "${CHAINS[@]}"; do
  read -r chain network coin sign <<<"$item"
  log "create address $chain/$network for user"
  user_json="$(create_addr "$chain" "$network" "$coin" "$sign")"
  log "create address $chain/$network for treasury"
  treasury_json="$(create_addr_treasury "$chain" "$network" "$coin" "$sign")"

  user_key_id="$(printf '%s' "$user_json" | extract_field key_id)"
  user_address="$(printf '%s' "$user_json" | extract_field address)"
  treasury_address="$(printf '%s' "$treasury_json" | extract_field address)"
  if [[ -z "$user_key_id" || -z "$user_address" || -z "$treasury_address" ]]; then
    echo "[e2e] missing key_id/address for $chain/$network" >&2
    exit 1
  fi

  log "deposit notify mock $chain/$network"
  REQ POST /v1/deposit/notify "{\"tenant_id\":\"$TENANT_ID\",\"account_id\":\"$USER_ACCOUNT_ID\",\"order_id\":\"dep_${chain}_${network}_001\",\"chain\":\"$chain\",\"network\":\"$network\",\"coin\":\"$coin\",\"amount\":\"1000000\",\"tx_hash\":\"0xdep_${chain}_${network}\",\"from_address\":\"faucet\",\"to_address\":\"$user_address\",\"confirmations\":1,\"required_confirmations\":1,\"status\":\"CONFIRMED\"}" >/dev/null

  log "withdraw mock $chain/$network"
  REQ POST /v1/withdraw "{\"tenant_id\":\"$TENANT_ID\",\"account_id\":\"$USER_ACCOUNT_ID\",\"order_id\":\"wd_${chain}_${network}_001\",\"chain\":\"$chain\",\"network\":\"$network\",\"coin\":\"$coin\",\"key_id\":\"$user_key_id\",\"sign_type\":\"$sign\",\"from\":\"$user_address\",\"to\":\"$treasury_address\",\"amount\":\"1\"}" >/dev/null

  log "sweep mock $chain/$network"
  REQ POST /v1/sweep/run "{\"tenant_id\":\"$TENANT_ID\",\"from_account_id\":\"$USER_ACCOUNT_ID\",\"treasury_account_id\":\"$TREASURY_ACCOUNT_ID\",\"chain\":\"$chain\",\"network\":\"$network\",\"asset\":\"$coin\",\"amount\":\"1\",\"sweep_order_id\":\"sw_${chain}_${network}_001\"}" >/dev/null

done

log "done. verify endpoints manually:"
echo "  $BASE_URL/v1/account/addresses?tenant_id=$TENANT_ID&account_id=$USER_ACCOUNT_ID"
echo "  $BASE_URL/v1/account/assets?tenant_id=$TENANT_ID&account_id=$USER_ACCOUNT_ID"
