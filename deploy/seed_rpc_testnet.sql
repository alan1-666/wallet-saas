-- Seed testnet RPC endpoints for wallet-saas chain-gateway control-plane.
-- Usage:
--   export ZAN_API_KEY="your_zan_api_key_here"
--   envsubst < deploy/seed_rpc_testnet.sql | psql "$WALLET_DB_DSN"

INSERT INTO rpc_endpoints (chain, network, model, endpoint_url, weight, timeout_ms, priority, status)
VALUES
  ('ethereum', 'sepolia', 'account', 'https://api.zan.top/node/v1/eth/sepolia/${ZAN_API_KEY}', 100, 12000, 100, 'ACTIVE'),
  ('solana',   'devnet',  'account', 'https://api.zan.top/node/v1/solana/devnet/${ZAN_API_KEY}', 100, 12000, 100, 'ACTIVE'),
  ('binance',  'testnet', 'account', 'https://api.zan.top/node/v1/bsc/testnet/${ZAN_API_KEY}', 100, 12000, 100, 'ACTIVE'),
  ('polygon',  'amoy',    'account', 'https://api.zan.top/node/v1/polygon/amoy/${ZAN_API_KEY}', 100, 12000, 100, 'ACTIVE'),
  ('arbitrum', 'sepolia', 'account', 'https://api.zan.top/node/v1/arb/sepolia/${ZAN_API_KEY}', 100, 12000, 100, 'ACTIVE'),
  ('tron',     'nile',    'account', 'https://api.zan.top/node/v1/tron/nile/${ZAN_API_KEY}/jsonrpc', 100, 12000, 100, 'ACTIVE')
ON CONFLICT (chain, network, model, endpoint_url)
DO UPDATE SET
  weight = EXCLUDED.weight,
  timeout_ms = EXCLUDED.timeout_ms,
  priority = EXCLUDED.priority,
  status = EXCLUDED.status,
  updated_at = NOW();
