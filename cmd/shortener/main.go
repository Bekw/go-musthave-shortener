package main

import (
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"strings"

	"github.com/Bekw/go-musthave-shortener/internal/config"

	"github.com/go-chi/chi/v5"
)

var urlStore = make(map[string]string)
var cfg *config.Config

func generateID() string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	b := make([]byte, 6)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func postHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "only POST allowed", http.StatusBadRequest)
		return
	}

	if r.Header.Get("Content-Type") != "text/plain" {
		http.Error(w, "Content-Type must be text/plain", http.StatusBadRequest)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		http.Error(w, "empty body", http.StatusBadRequest)
		return
	}
	originalURL := strings.TrimSpace(string(body))

	id := generateID()
	urlStore[id] = originalURL

	shortURL := fmt.Sprintf("%s/%s", cfg.BaseURL, id)

	w.WriteHeader(http.StatusCreated)
	w.Header().Set("Content-Type", "text/plain")
	_, _ = w.Write([]byte(shortURL))
}

func getHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}

	originalURL, ok := urlStore[id]
	if !ok {
		http.Error(w, "id not found", http.StatusBadRequest)
		return
	}

	w.Header().Set("Location", originalURL)
	w.WriteHeader(http.StatusTemporaryRedirect)
}

func main() {
	cfg = config.NewConfig()

	r := chi.NewRouter()

	r.Post("/", postHandler)
	r.Get("/{id}", getHandler)

	fmt.Printf("Server started at %s\n", cfg.Address)
	log.Fatal(http.ListenAndServe(cfg.Address, r))
}
