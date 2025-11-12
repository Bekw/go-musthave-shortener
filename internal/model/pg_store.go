package model

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type PGStore struct {
	db *sql.DB
}

func NewPGStore(db *sql.DB) *PGStore {
	return &PGStore{db: db}
}

func (s *PGStore) Save(id, url string) error {
	const q = `
		INSERT INTO urls (id, original_url)
		VALUES ($1, $2)
		ON CONFLICT (original_url)
		DO UPDATE SET original_url = EXCLUDED.original_url
		RETURNING id;
	`

	var retID string
	if err := s.db.QueryRowContext(context.Background(), q, id, url).Scan(&retID); err != nil {
		return err
	}
	if retID != id {
		return &DuplicateURLError{ExistingID: retID}
	}
	return nil
}

func (s *PGStore) Get(id string) (string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	var original string
	err := s.db.QueryRowContext(ctx,
		`SELECT original_url FROM urls WHERE id = $1`, id,
	).Scan(&original)
	if errors.Is(err, sql.ErrNoRows) || err != nil {
		return "", false
	}
	return original, true
}

func EnsureSchema(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `
        CREATE TABLE IF NOT EXISTS urls (
            id           TEXT PRIMARY KEY,
            original_url TEXT NOT NULL,
            created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
        );
    `); err != nil {
		return err
	}

	if _, err := db.ExecContext(ctx, `
        DO $$
        BEGIN
            IF NOT EXISTS (
                SELECT 1 FROM pg_indexes WHERE schemaname = 'public' AND indexname = 'urls_original_url_key'
            ) THEN
                ALTER TABLE urls
                    ADD CONSTRAINT urls_original_url_key UNIQUE (original_url);
            END IF;
        END $$;
    `); err != nil {
		return err
	}
	return nil
}
