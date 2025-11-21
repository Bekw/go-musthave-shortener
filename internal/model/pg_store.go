package model

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
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
		ON CONFLICT (id) DO NOTHING;
	`

	res, err := s.db.ExecContext(ctx, q, id, url)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			existID, ok, e := s.FindByOriginal(ctx, url)
			if e != nil {
				return e
			}
			if ok {
				return &DuplicateURLError{ExistingID: existID}
			}
			return ErrDuplicateOriginal
		}
		return err
	}

	aff, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if aff == 0 {
		return ErrCollision
	}

	return nil
}

func (s *PGStore) Get(ctx context.Context, id string) (string, bool, error) {
	var u string
	var deleted bool

	err := s.db.QueryRowContext(ctx,
		`SELECT original_url, is_deleted FROM urls WHERE id = $1`, id,
	).Scan(&u, &deleted)

	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if deleted {
		return "", false, ErrDeleted
	}

	return u, true, nil
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
            created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
            is_deleted   BOOLEAN NOT NULL DEFAULT FALSE
        );
    `); err != nil {
		return err
	}

	if _, err := db.ExecContext(ctx, `
        ALTER TABLE urls
        ADD COLUMN IF NOT EXISTS is_deleted BOOLEAN NOT NULL DEFAULT FALSE;
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
func (s *PGStore) MarkDeleted(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	uniq := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		uniq = append(uniq, id)
	}

	placeholders := make([]string, len(uniq))
	args := make([]any, len(uniq))
	for i, id := range uniq {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}

	query := fmt.Sprintf(
		`UPDATE urls SET is_deleted = TRUE WHERE id IN (%s);`,
		strings.Join(placeholders, ","),
	)

	_, err := s.db.ExecContext(ctx, query, args...)
	return err
}
