-- +goose Up
CREATE TABLE IF NOT EXISTS user_urls (
    user_id TEXT NOT NULL,
    url_id  TEXT NOT NULL,
    PRIMARY KEY (user_id, url_id),
    FOREIGN KEY (url_id) REFERENCES urls(id)
);
CREATE INDEX IF NOT EXISTS idx_user_urls_user_id ON user_urls(user_id);

-- +goose Down
DROP INDEX IF EXISTS idx_user_urls_user_id;
DROP TABLE IF EXISTS user_urls;
