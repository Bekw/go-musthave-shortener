package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
	"go.uber.org/zap"

	"github.com/Bekw/go-musthave-shortener/internal/config"
	"github.com/Bekw/go-musthave-shortener/internal/handler"
	appmw "github.com/Bekw/go-musthave-shortener/internal/middleware"
	"github.com/Bekw/go-musthave-shortener/internal/model"
)

func mustInitDB(dsn string, logger *zap.Logger) *sql.DB {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		logger.Fatal("db open failed", zap.Error(err))
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		logger.Fatal("db initial ping failed", zap.Error(err))
	}
	return db
}

func main() {
	cfg := config.FromFlags()

	logger, _ := zap.NewProduction()
	defer logger.Sync()

	var store model.Store

	if cfg.DatabaseDSN != "" {
		db := mustInitDB(cfg.DatabaseDSN, logger)

		if err := model.EnsureSchema(context.Background(), db); err != nil {
			logger.Fatal("db ensure schema failed", zap.Error(err))
		}

		store = model.NewPGStore(db)
	} else if cfg.FilePath != "" {
		fs, err := model.NewFileStore(cfg.FilePath)
		if err != nil {
			log.Fatalf("init file store: %v", err)
		}
		store = fs
	} else {
		store = model.NewMemoryStore()
	}

	var db *sql.DB
	if cfg.DatabaseDSN != "" {
		db = mustInitDB(cfg.DatabaseDSN, logger)
	}
	h := handler.NewHandler(store, cfg.BaseURL, logger)
	h.SetDB(db)

	r := chi.NewRouter()
	r.Use(appmw.Logger(logger))
	r.Use(appmw.Gzip())

	r.Post("/", h.PostHandler)
	r.Post("/api/shorten", h.PostJSONHandler)
	r.Get("/{id}", h.GetHandler)
	r.Get("/ping", h.PingHandler)

	log.Printf("Server running on %s", cfg.Address)
	log.Fatal(http.ListenAndServe(cfg.Address, r))
}
