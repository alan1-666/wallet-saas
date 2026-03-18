-- Seed testnet RPC endpoints for wallet-saas chain-gateway control-plane.
-- Usage:
--   psql "$WALLET_DB_DSN" -f deploy/seed_rpc_testnet.sql

INSERT INTO rpc_endpoints (chain, network, model, endpoint_url, weight, timeout_ms, priority, status)
VALUES
  ('ethereum', 'sepolia', 'account', 'https://api.zan.top/node/v1/eth/sepolia/055c525c14694e578f53eaf8ccf1254c', 100, 12000, 100, 'ACTIVE'),
  ('solana',   'devnet',  'account', 'https://api.zan.top/node/v1/solana/devnet/055c525c14694e578f53eaf8ccf1254c', 100, 12000, 100, 'ACTIVE'),
  ('binance',  'testnet', 'account', 'https://api.zan.top/node/v1/bsc/testnet/055c525c14694e578f53eaf8ccf1254c', 100, 12000, 100, 'ACTIVE'),
  ('polygon',  'amoy',    'account', 'https://api.zan.top/node/v1/polygon/amoy/055c525c14694e578f53eaf8ccf1254c', 100, 12000, 100, 'ACTIVE'),
  ('arbitrum', 'sepolia', 'account', 'https://api.zan.top/node/v1/arb/sepolia/055c525c14694e578f53eaf8ccf1254c', 100, 12000, 100, 'ACTIVE'),
  ('tron',     'nile',    'account', 'https://api.zan.top/node/v1/tron/nile/055c525c14694e578f53eaf8ccf1254c/jsonrpc', 100, 12000, 100, 'ACTIVE')
ON CONFLICT (chain, network, model, endpoint_url)
DO UPDATE SET
  weight = EXCLUDED.weight,
  timeout_ms = EXCLUDED.timeout_ms,
  priority = EXCLUDED.priority,
  status = EXCLUDED.status,
  updated_at = NOW();
