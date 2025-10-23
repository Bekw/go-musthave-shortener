package handler

import (
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/Bekw/go-musthave-shortener/internal/model"
)

type Handler struct {
	store   model.Store
	baseURL string
}

func generateID() string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	b := make([]byte, 6)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func NewHandler(store model.Store, baseURL string) *Handler {
	return &Handler{store: store, baseURL: strings.TrimSuffix(baseURL, "/")}
}

func (h *Handler) PostHandler(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Content-Type") != "text/plain" {
		http.Error(w, "Content-Type must be text/plain", http.StatusBadRequest)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		http.Error(w, "empty body", http.StatusBadRequest)
		return
	}

	original := strings.TrimSpace(string(body))
	id := generateID()

	for h.store.Exists(id) {
		id = generateID()
	}

	h.store.Set(id, original)

	shortURL, err := url.JoinPath(h.baseURL, id)
	if err != nil {
		http.Error(w, "bad base url", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusCreated)
	if _, err := w.Write([]byte(shortURL)); err != nil {
		log.Printf("write response error: %v", err)
	}
}

func (h *Handler) GetHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}

	original, ok := h.store.Get(id)
	if !ok {
		http.Error(w, "id not found", http.StatusBadRequest)
		return
	}

	w.Header().Set("Location", original)
	w.WriteHeader(http.StatusTemporaryRedirect)
}
