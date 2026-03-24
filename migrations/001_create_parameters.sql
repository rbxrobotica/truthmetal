-- Parameters: immutable versioned key-value facts.
-- Once inserted, rows are only updated via the state column.
-- The value itself is never modified after insertion.

CREATE TABLE IF NOT EXISTS parameters (
    id                  UUID PRIMARY KEY,
    key                 TEXT NOT NULL,
    value               TEXT NOT NULL,
    value_type          TEXT NOT NULL DEFAULT 'string',
    namespace           TEXT NOT NULL,
    source              TEXT NOT NULL DEFAULT '',
    source_ref          TEXT NOT NULL DEFAULT '',
    version             BIGINT NOT NULL DEFAULT 1,
    previous_version_id UUID REFERENCES parameters(id),
    state               TEXT NOT NULL DEFAULT 'SUBMITTED',
    metadata            JSONB NOT NULL DEFAULT '{}',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Canonical lookup: namespace + key → latest canonical version
CREATE UNIQUE INDEX IF NOT EXISTS idx_parameters_canonical
    ON parameters (namespace, key)
    WHERE state = 'CANONICAL';

-- Version history lookup
CREATE INDEX IF NOT EXISTS idx_parameters_ns_key_version
    ON parameters (namespace, key, version);
