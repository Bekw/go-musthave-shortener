package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"go.uber.org/zap"

	"github.com/Bekw/go-musthave-shortener/internal/model"
)

type Handler struct {
	store   model.Store
	baseURL string
	log     *zap.Logger
	db      *sql.DB
}

type shortenRequest struct {
	URL string `json:"url"`
}

type shortenResponse struct {
	Result string `json:"result"`
}

type batchReqItem struct {
	CorrelationID string `json:"correlation_id"`
	OriginalURL   string `json:"original_url"`
}

type batchResItem struct {
	CorrelationID string `json:"correlation_id"`
	ShortURL      string `json:"short_url"`
}

func generateID() string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	b := make([]byte, 6)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func NewHandler(store model.Store, baseURL string, log *zap.Logger) *Handler {
	if log == nil {
		log = zap.NewNop()
	}
	return &Handler{store: store, baseURL: baseURL, log: log}
}

func (h *Handler) SetDB(db *sql.DB) {
	h.db = db
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

	const maxAttempts = 5
	var id string
	for attempt := 0; attempt < maxAttempts; attempt++ {
		try := generateID()
		if err := h.store.Save(try, original); err != nil {
			var dup *model.DuplicateURLError
			if errors.As(err, &dup) {
				short, _ := url.JoinPath(h.baseURL, dup.ExistingID)
				w.Header().Set("Content-Type", "text/plain")
				w.WriteHeader(http.StatusConflict)
				_, _ = w.Write([]byte(short))
				return
			}
			if errors.Is(err, model.ErrCollision) {
				continue
			}
			http.Error(w, "storage error", http.StatusInternalServerError)
			return
		}
		id = try
		break
	}
	if id == "" {
		http.Error(w, "cannot allocate id", http.StatusInternalServerError)
		return
	}

	shortURL, err := url.JoinPath(h.baseURL, id)
	if err != nil {
		http.Error(w, "bad base url", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusCreated)
	if _, err := w.Write([]byte(shortURL)); err != nil {
		h.log.Error("response error: %v", zap.Error(err))
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

func (h *Handler) PostJSONHandler(w http.ResponseWriter, r *http.Request) {
	if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		http.Error(w, "Content-Type must be application/json", http.StatusBadRequest)
		return
	}

	var req shortenRequest
	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		http.Error(w, "empty body", http.StatusBadRequest)
		return
	}
	if err := json.Unmarshal(body, &req); err != nil || strings.TrimSpace(req.URL) == "" {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	original := strings.TrimSpace(req.URL)

	const maxAttempts = 5
	var id string
	for attempt := 0; attempt < maxAttempts; attempt++ {
		try := generateID()
		if err := h.store.Save(try, original); err != nil {
			var dup *model.DuplicateURLError
			if errors.As(err, &dup) {
				short, _ := url.JoinPath(h.baseURL, dup.ExistingID)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusConflict)
				_ = json.NewEncoder(w).Encode(shortenResponse{Result: short})
				return
			}
			if errors.Is(err, model.ErrCollision) {
				continue
			}
			http.Error(w, "storage error", http.StatusInternalServerError)
			return
		}
		id = try
		break
	}
	if id == "" {
		http.Error(w, "cannot allocate id", http.StatusInternalServerError)
		return
	}

	shortURL, err := url.JoinPath(h.baseURL, id)
	if err != nil {
		http.Error(w, "bad base url", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(shortenResponse{Result: shortURL}); err != nil {
		h.log.Error("encoding error: %v", zap.Error(err))
	}
}

func (h *Handler) PingHandler(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		w.WriteHeader(http.StatusOK)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), time.Second)
	defer cancel()
	if err := h.db.PingContext(ctx); err != nil {
		h.log.Error("db ping failed", zap.Error(err))
		http.Error(w, "db unavailable", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}
func (h *Handler) PostBatchHandler(w http.ResponseWriter, r *http.Request) {
	if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		http.Error(w, "Content-Type must be application/json", http.StatusBadRequest)
		return
	}

	var in []batchReqItem
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if len(in) == 0 {
		http.Error(w, "empty batch", http.StatusBadRequest)
		return
	}

	out := make([]batchResItem, 0, len(in))

	for _, it := range in {
		urlStr := strings.TrimSpace(it.OriginalURL)
		if urlStr == "" || it.CorrelationID == "" {
			http.Error(w, "empty url or correlation_id", http.StatusBadRequest)
			return
		}

		const maxAttempts = 5
		var id string
		for attempt := 0; attempt < maxAttempts; attempt++ {
			try := generateID()
			if err := h.store.Save(try, urlStr); err != nil {
				if errors.Is(err, model.ErrCollision) {
					continue
				}
				http.Error(w, "storage error", http.StatusInternalServerError)
				return
			}
			id = try
			break
		}
		if id == "" {
			http.Error(w, "cannot allocate id", http.StatusInternalServerError)
			return
		}

		short, err := url.JoinPath(h.baseURL, id)
		if err != nil {
			http.Error(w, "bad base url", http.StatusBadRequest)
			return
		}

		out = append(out, batchResItem{
			CorrelationID: it.CorrelationID,
			ShortURL:      short,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(out)
}
