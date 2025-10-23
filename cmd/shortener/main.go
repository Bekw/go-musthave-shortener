package main

import (
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/Bekw/go-musthave-shortener/internal/config"
	"github.com/Bekw/go-musthave-shortener/internal/handler"
	"github.com/Bekw/go-musthave-shortener/internal/model"
)

func main() {
	cfg := config.NewConfig()

	store := model.NewMemoryStore()
	h := handler.NewHandler(store, cfg.BaseURL)

	r := chi.NewRouter()
	r.Post("/", h.PostHandler)
	r.Get("/{id}", h.GetHandler)

	log.Printf("Server running on %s", cfg.Address)
	log.Fatal(http.ListenAndServe(cfg.Address, r))
}
