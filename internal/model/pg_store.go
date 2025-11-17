package model

import (
	"context"
	"database/sql"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgerrcode"
)

type PGStore struct {
	db *sql.DB
}

func NewPGStore(db *sql.DB) *PGStore {
	return &PGStore{db: db}
}

func (s *PGStore) Save(ctx context.Context, id, url string) error {
	const q = `
INSERT INTO urls (id, original_url)
VALUES ($1, $2)
ON CONFLICT (id) DO NOTHING;`

	res, err := s.db.ExecContext(ctx, q, id, url)
	if err != nil {
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == pgerrcode.UniqueViolation {
			return ErrCollision
		}
		return err
	}
	aff, _ := res.RowsAffected()
	if aff == 1 {
		return nil
	}

	var existingID string
	err = s.db.QueryRowContext(ctx,
		`SELECT id FROM urls WHERE original_url = $1`, url,
	).Scan(&existingID)
	if err == nil {
		return ErrDuplicateOriginal
	}
	if err == sql.ErrNoRows {
		return ErrCollision
	}
	return err
}

func (s *PGStore) Get(ctx context.Context, id string) (string, bool, error) {
	var u string
	err := s.db.QueryRowContext(ctx,
		`SELECT original_url FROM urls WHERE id = $1`, id,
	).Scan(&u)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	return u, err == nil, err
}

func (s *PGStore) FindByOriginal(ctx context.Context, url string) (string, bool, error) {
	var id string
	err := s.db.QueryRowContext(ctx,
		`SELECT id FROM urls WHERE original_url = $1`, url,
	).Scan(&id)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	return id, err == nil, err
}

func (s *PGStore) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
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
		CREATE UNIQUE INDEX IF NOT EXISTS urls_original_url_idx
		ON urls (original_url);
	`); err != nil {
		return err
	}

	return nil
}
