-- Ledger: append-only audit trail of every state transition.
-- Rows are NEVER updated or deleted.

CREATE TABLE IF NOT EXISTS ledger_events (
    id           UUID PRIMARY KEY,
    parameter_id UUID NOT NULL REFERENCES parameters(id),
    event_type   TEXT NOT NULL,
    from_state   TEXT NOT NULL DEFAULT '',
    to_state     TEXT NOT NULL,
    actor        TEXT NOT NULL DEFAULT 'system',
    reason       TEXT NOT NULL DEFAULT '',
    context      JSONB NOT NULL DEFAULT '{}',
    occurred_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ledger_parameter_id
    ON ledger_events (parameter_id, occurred_at DESC);
