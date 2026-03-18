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

extract_key_id() {
  python3 - <<'PY'
import json,sys
obj=json.load(sys.stdin)
print(obj.get("key_id",""))
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
  create_addr "$chain" "$network" "$coin" "$sign" >/dev/null
  log "create address $chain/$network for treasury"
  create_addr_treasury "$chain" "$network" "$coin" "$sign" >/dev/null

  log "deposit notify mock $chain/$network"
  REQ POST /v1/deposit/notify "{\"tenant_id\":\"$TENANT_ID\",\"account_id\":\"$USER_ACCOUNT_ID\",\"order_id\":\"dep_${chain}_${network}_001\",\"chain\":\"$chain\",\"network\":\"$network\",\"coin\":\"$coin\",\"amount\":\"1000000\",\"tx_hash\":\"0xdep_${chain}_${network}\",\"from_address\":\"from\",\"to_address\":\"to\",\"confirmations\":1,\"required_confirmations\":1,\"status\":\"CONFIRMED\"}" >/dev/null

  key_json="$(create_addr "$chain" "$network" "$coin" "$sign")"
  key_id="$(printf '%s' "$key_json" | extract_key_id)"
  if [[ -n "$key_id" ]]; then
    log "withdraw mock $chain/$network"
    REQ POST /v1/withdraw "{\"tenant_id\":\"$TENANT_ID\",\"account_id\":\"$USER_ACCOUNT_ID\",\"order_id\":\"wd_${chain}_${network}_001\",\"chain\":\"$chain\",\"network\":\"$network\",\"coin\":\"$coin\",\"key_id\":\"$key_id\",\"sign_type\":\"$sign\",\"from\":\"from\",\"to\":\"to\",\"amount\":\"1\"}" >/dev/null || true
  fi

  log "sweep mock $chain/$network"
  REQ POST /v1/sweep/run "{\"tenant_id\":\"$TENANT_ID\",\"from_account_id\":\"$USER_ACCOUNT_ID\",\"treasury_account_id\":\"$TREASURY_ACCOUNT_ID\",\"chain\":\"$chain\",\"network\":\"$network\",\"asset\":\"$coin\",\"amount\":\"1\",\"sweep_order_id\":\"sw_${chain}_${network}_001\"}" >/dev/null || true

done

log "done. verify endpoints manually:"
echo "  $BASE_URL/v1/account/addresses?tenant_id=$TENANT_ID&account_id=$USER_ACCOUNT_ID"
echo "  $BASE_URL/v1/account/assets?tenant_id=$TENANT_ID&account_id=$USER_ACCOUNT_ID"
