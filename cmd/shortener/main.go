package main

import (
	"flag"
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
	addrFlag := flag.String("a", config.DefaultAddress, "HTTP server address")
	baseFlag := flag.String("b", config.DefaultBaseURL, "Base URL for short links")
	flag.Parse()

	logger, _ := zap.NewProduction()
	defer logger.Sync()

	cfg := config.NewConfig(*addrFlag, *baseFlag)

	store := model.NewMemoryStore()
	h := handler.NewHandler(store, cfg.BaseURL)

	r := chi.NewRouter()
	r.Use(appmw.Logger(logger))
	r.Use(appmw.Gzip())

	r.Post("/", h.PostHandler)
	r.Get("/{id}", h.GetHandler)
	// 7 - инкремент
	r.Post("/api/shorten", h.PostJSONHandler)

	log.Printf("Server running on %s", cfg.Address)
	log.Fatal(http.ListenAndServe(cfg.Address, r))
}
