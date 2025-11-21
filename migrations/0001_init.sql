-- +goose Up
CREATE TABLE IF NOT EXISTS urls (
    id           VARCHAR(32) PRIMARY KEY,
    original_url TEXT        NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_urls_created_at ON urls (created_at);

-- +goose Down
DROP INDEX IF EXISTS idx_urls_created_at;
DROP TABLE IF EXISTS urls;
