CREATE TABLE IF NOT EXISTS accounts (
 id bigint PRIMARY KEY,
 balance_minor bigint NOT NULL CHECK (balance_minor >= 0),
 initial_balance bigint NOT NULL CHECK (initial_balance >= 0),
 last_sequence bigint NOT NULL DEFAULT 0 CHECK(last_sequence >= 0)
);
INSERT INTO accounts(id,balance_minor,initial_balance)
 SELECT -i,0,0 FROM generate_series(1,32) i ON CONFLICT DO NOTHING;
CREATE TABLE IF NOT EXISTS operations (
 id text PRIMARY KEY,
 payload jsonb NOT NULL,
 decision text NOT NULL CHECK(decision IN ('PROCESSING','APPROVED','DECLINED')),
 reason text NOT NULL DEFAULT '',
 settlement text NOT NULL DEFAULT 'INTERNAL'
  CHECK(settlement IN ('INTERNAL','PENDING','SETTLED','REVERSED')),
 reverses_id text UNIQUE REFERENCES operations(id),
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS entries (
 operation_id text NOT NULL REFERENCES operations(id),
 account_id bigint NOT NULL REFERENCES accounts(id),
 delta_minor bigint NOT NULL CHECK(delta_minor <> 0),
 account_sequence bigint NOT NULL,
 balance_after_minor bigint NOT NULL CHECK(balance_after_minor >= 0),
 PRIMARY KEY(operation_id,account_id),
 UNIQUE(account_id,account_sequence)
);
CREATE TABLE IF NOT EXISTS outbox (
 operation_id text PRIMARY KEY REFERENCES operations(id),
 lease_until timestamptz NOT NULL DEFAULT '-infinity',
 attempts integer NOT NULL DEFAULT 0,
 done boolean NOT NULL DEFAULT false
);
CREATE INDEX IF NOT EXISTS outbox_pending ON outbox(lease_until) WHERE NOT done;
CREATE TABLE IF NOT EXISTS external_receipts (
 id text PRIMARY KEY, payload jsonb NOT NULL, decision text NOT NULL,
 amount_minor bigint NOT NULL CHECK(amount_minor > 0)
);
CREATE OR REPLACE FUNCTION forbid_entry_change() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'entries are immutable'; END $$;
DROP TRIGGER IF EXISTS entries_immutable ON entries;
CREATE TRIGGER entries_immutable BEFORE UPDATE OR DELETE ON entries
 FOR EACH ROW EXECUTE FUNCTION forbid_entry_change();
ALTER TABLE accounts SET (
 fillfactor=70,
 autovacuum_vacuum_scale_factor=0,
 autovacuum_vacuum_threshold=500,
 autovacuum_vacuum_cost_delay=0
);
