package main

import (
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/Bekw/go-musthave-shortener/internal/config"
	"github.com/Bekw/go-musthave-shortener/internal/handler"
	appmw "github.com/Bekw/go-musthave-shortener/internal/middleware"
	"github.com/Bekw/go-musthave-shortener/internal/model"
)

func main() {
	cfg := config.FromFlags()

	var store model.Store
	var err error
	if cfg.FilePath != "" {
		store, err = model.NewFileStore(cfg.FilePath)
		if err != nil {
			log.Fatalf("init file store: %v", err)
		}
	} else {
		store = model.NewMemoryStore()
	}

	logger, _ := zap.NewProduction()
	defer logger.Sync()

	h := handler.NewHandler(store, cfg.BaseURL, logger)

	r := chi.NewRouter()
	r.Use(appmw.Logger(logger))
	r.Use(appmw.Gzip())

	r.Post("/", h.PostHandler)
	r.Post("/api/shorten", h.PostJSONHandler)
	r.Get("/{id}", h.GetHandler)

	log.Printf("Server running on %s", cfg.Address)
	log.Fatal(http.ListenAndServe(cfg.Address, r))
}
