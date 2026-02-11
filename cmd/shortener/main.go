package main

import (
	"context"
	"database/sql"
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

func mustInitDB(ctx context.Context, dsn string) *sql.DB {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}

	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("db ping failed: %v", err)
	}

	if err := model.EnsureSchema(ctx, db); err != nil {
		log.Fatalf("ensure schema failed: %v", err)
	}

	return db
}

func main() {
	cfg := config.FromFlags()
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	ctx := context.Background()

	var store model.Store

	switch {
	case cfg.DatabaseDSN != "":
		db := mustInitDB(ctx, cfg.DatabaseDSN)
		store = model.NewPGStore(db)
		logger.Info("using PostgreSQL storage", zap.String("dsn", cfg.DatabaseDSN))

	case cfg.FilePath != "":
		fs, err := model.NewFileStore(cfg.FilePath)
		if err != nil {
			logger.Fatal("init file store", zap.Error(err))
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

	aud := audit.New()
	aud.Add(audit.NewFileSink(cfg.AuditFile))
	aud.Add(audit.NewHTTPSink(cfg.AuditURL, nil))

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
