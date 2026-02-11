package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
	"go.uber.org/zap"

	"github.com/Bekw/go-musthave-shortener/internal/audit"
	"github.com/Bekw/go-musthave-shortener/internal/config"
	"github.com/Bekw/go-musthave-shortener/internal/handler"
	appmw "github.com/Bekw/go-musthave-shortener/internal/middleware"
	"github.com/Bekw/go-musthave-shortener/internal/model"
)

func initDB(ctx context.Context, dsn string) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("db ping failed: %w", err)
	}

	if err := model.EnsureSchema(ctx, db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ensure schema failed: %w", err)
	}

	return db, nil
}

func main() {
	cfg := config.FromFlags()
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	ctx := context.Background()

	var store model.Store

	switch {
	case cfg.DatabaseDSN != "":
		db, err := initDB(ctx, cfg.DatabaseDSN)
		if err != nil {
			log.Fatal(err)
		}
		store = model.NewPGStore(db)
		logger.Info("using PostgreSQL storage", zap.String("dsn", cfg.DatabaseDSN))

	case cfg.FilePath != "":
		fs, err := model.NewFileStore(cfg.FilePath)
		if err != nil {
			log.Fatal(err)
		}
		store = fs
		logger.Info("using file storage", zap.String("path", cfg.FilePath))

	default:
		store = model.NewMemoryStore()
		logger.Info("using in-memory storage")
	}

	aud := audit.New()
	if cfg.AuditFile != "" {
		aud.Add(audit.NewFileSink(cfg.AuditFile))
		logger.Info("audit file sink enabled", zap.String("file", cfg.AuditFile))
	}
	if cfg.AuditURL != "" {
		aud.Add(audit.NewHTTPSink(cfg.AuditURL, nil))
		logger.Info("audit HTTP sink enabled", zap.String("url", cfg.AuditURL))
	}

	h := handler.NewHandler(store, cfg.BaseURL, logger)
	h.SetAuditor(aud)

	r := chi.NewRouter()
	r.Use(appmw.Logger(logger))
	r.Use(appmw.Gzip())

	r.Get("/ping", h.PingHandler)
	r.Post("/", h.PostHandler)
	r.Post("/api/shorten", h.PostJSONHandler)
	r.Post("/api/shorten/batch", h.PostBatchHandler)
	r.Get("/api/user/urls", h.GetUserURLsHandler)
	r.Delete("/api/user/urls", h.DeleteUserURLsHandler)
	r.Get("/{id}", h.GetHandler)

	logger.Info("server started", zap.String("addr", cfg.Address))
	log.Fatal(http.ListenAndServe(cfg.Address, r))
}
