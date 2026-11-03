-- import_requests makes POST /ads/import idempotent: a retry with the same Idempotency-Key replays the
-- recorded result instead of creating the ads again.
--
-- The row is inserted at the start of the import transaction, so it is committed or rolled back together
-- with the ads, and a concurrent request with the same key waits on the primary key until the first one ends.
CREATE TABLE import_requests (
    user_id         UUID        NOT NULL,
    idempotency_key UUID        NOT NULL,
    -- request_hash is the SHA-256 of the imported ads: the same key sent with another content is rejected.
    request_hash    BYTEA       NOT NULL CHECK (octet_length(request_hash) = 32),
    -- ad_ids is filled once the ads are created, in the same transaction.
    ad_ids          BIGINT[],
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, idempotency_key)
);

-- Supports purging the keys past their retention period.
CREATE INDEX import_requests_created_at_idx ON import_requests (created_at);
