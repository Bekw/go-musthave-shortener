package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
	"go.uber.org/zap"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"

	"github.com/Bekw/go-musthave-shortener/internal/audit"
	"github.com/Bekw/go-musthave-shortener/internal/config"
	"github.com/Bekw/go-musthave-shortener/internal/grpcapi"
	"github.com/Bekw/go-musthave-shortener/internal/handler"
	appmw "github.com/Bekw/go-musthave-shortener/internal/middleware"
	"github.com/Bekw/go-musthave-shortener/internal/model"
)

const defaultShutdownTimeout = 10 * time.Second

var (
	buildVersion string
	buildDate    string
	buildCommit  string
)

func printBuildInfo() {
	v := buildVersion
	d := buildDate
	c := buildCommit

	if v == "" {
		v = "N/A"
	}
	if d == "" {
		d = "N/A"
	}
	if c == "" {
		c = "N/A"
	}

	fmt.Printf("Build version: %s\n", v)
	fmt.Printf("Build date: %s\n", d)
	fmt.Printf("Build commit: %s\n", c)
}

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

type closer interface {
	Close() error
}

func main() {
	printBuildInfo()

	cfg, err := config.FromFlags()
	if err != nil {
		log.Fatal(err)
	}

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
	if err := h.SetTrustedSubnet(cfg.TrustedSubnet); err != nil {
		log.Fatal(err)
	}

	grpcImpl := grpcapi.New(store, cfg.BaseURL, logger, []byte(cfg.SecretKey))
	grpcImpl.SetAuditor(aud)

	grpcSrv := grpc.NewServer()
	grpcImpl.Register(grpcSrv)

	r := chi.NewRouter()
	r.Use(appmw.Logger(logger))
	r.Use(appmw.Gzip())

	r.Get("/ping", h.PingHandler)
	r.Post("/", h.PostHandler)
	r.Post("/api/shorten", h.PostJSONHandler)
	r.Post("/api/shorten/batch", h.PostBatchHandler)
	r.Get("/api/user/urls", h.GetUserURLsHandler)
	r.Delete("/api/user/urls", h.DeleteUserURLsHandler)
	r.Get("/api/internal/stats", h.InternalStatsHandler)
	r.Get("/{id}", h.GetHandler)

	mux := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.ProtoMajor == 2 && strings.HasPrefix(req.Header.Get("Content-Type"), "application/grpc") {
			grpcSrv.ServeHTTP(w, req)
			return
		}
		r.ServeHTTP(w, req)
	})

	var handlerWithH2 http.Handler = mux
	if !cfg.EnableHTTPS {
		handlerWithH2 = h2c.NewHandler(handlerWithH2, &http2.Server{})
	}

	srv := &http.Server{
		Addr:    cfg.Address,
		Handler: handlerWithH2,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("server started", zap.String("addr", cfg.Address))
		if cfg.EnableHTTPS {
			errCh <- srv.ListenAndServeTLS("cert.pem", "key.pem")
			return
		}
		errCh <- srv.ListenAndServe()
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	defer signal.Stop(sigCh)

	select {
	case sig := <-sigCh:
		logger.Info("shutdown signal received", zap.String("signal", sig.String()))
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			logger.Fatal("server error", zap.Error(err))
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), defaultShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("http server shutdown error", zap.Error(err))
	}
	grpcSrv.GracefulStop()

	if err := h.Shutdown(shutdownCtx); err != nil {
		logger.Error("handler shutdown error", zap.Error(err))
	}
	if err := grpcImpl.Shutdown(shutdownCtx); err != nil {
		logger.Error("grpc handler shutdown error", zap.Error(err))
	}

	if c, ok := store.(closer); ok {
		if err := c.Close(); err != nil {
			logger.Error("store close error", zap.Error(err))
		}
	}

	logger.Info("server stopped")
}
