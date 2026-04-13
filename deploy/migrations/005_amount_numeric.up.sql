-- Convert amount/balance TEXT columns to NUMERIC for arithmetic safety.
-- Use NUMERIC(78,0) to support full uint256 range (2^256-1 is 78 digits).
-- ALTER TYPE with USING is safe for columns that already contain valid numeric strings.

ALTER TABLE wallet_balances
  ALTER COLUMN balance TYPE NUMERIC(78,0) USING CASE WHEN balance ~ '^\d+$' THEN balance::NUMERIC ELSE 0 END,
  ALTER COLUMN frozen  TYPE NUMERIC(78,0) USING CASE WHEN frozen  ~ '^\d+$' THEN frozen::NUMERIC  ELSE 0 END;

ALTER TABLE wallet_ledger
  ALTER COLUMN amount         TYPE NUMERIC(78,0) USING CASE WHEN amount         ~ '^\d+$' THEN amount::NUMERIC         ELSE 0 END,
  ALTER COLUMN balance_before TYPE NUMERIC(78,0) USING CASE WHEN balance_before ~ '^\d+$' THEN balance_before::NUMERIC ELSE 0 END,
  ALTER COLUMN balance_after  TYPE NUMERIC(78,0) USING CASE WHEN balance_after  ~ '^\d+$' THEN balance_after::NUMERIC  ELSE 0 END;

ALTER TABLE wallet_withdraw_orders
  ALTER COLUMN amount TYPE NUMERIC(78,0) USING CASE WHEN amount ~ '^\d+$' THEN amount::NUMERIC ELSE 0 END,
  ALTER COLUMN fee    TYPE NUMERIC(78,0) USING CASE WHEN fee    ~ '^\d+$' THEN fee::NUMERIC    ELSE 0 END;

ALTER TABLE wallet_sweep_orders
  ALTER COLUMN amount TYPE NUMERIC(78,0) USING CASE WHEN amount ~ '^\d+$' THEN amount::NUMERIC ELSE 0 END;

ALTER TABLE scan_seen_events
  ALTER COLUMN amount TYPE NUMERIC(78,0) USING CASE WHEN amount ~ '^\d+$' THEN amount::NUMERIC ELSE 0 END;
