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
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	res, err := s.db.ExecContext(ctx, `
		INSERT INTO urls (id, original_url) VALUES ($1, $2)
		ON CONFLICT (id) DO NOTHING
	`, id, url)
	if err != nil {
		return err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return ErrCollision
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
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS urls (
			id           TEXT PRIMARY KEY,
			original_url TEXT NOT NULL,
			created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
		);
	`)
	return err
}
