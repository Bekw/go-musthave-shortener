package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"

	"go.uber.org/zap"

	"github.com/Bekw/go-musthave-shortener/internal/model"
)

type Handler struct {
	store   model.Store
	baseURL string
	log     *zap.Logger
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

func (h *Handler) PostHandler(w http.ResponseWriter, r *http.Request) {
	if ct := r.Header.Get("Content-Type"); ct != "text/plain" {
		http.Error(w, "Content-Type must be text/plain", http.StatusBadRequest)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		http.Error(w, "empty body", http.StatusBadRequest)
		return
	}
	original := strings.TrimSpace(string(body))

	id, existed, err := h.saveWithRetries(r.Context(), original, 5)
	if err != nil {
		h.log.Error("store save error", zap.Error(err))
		http.Error(w, "storage error", http.StatusInternalServerError)
		return
	}

	shortURL, _ := url.JoinPath(h.baseURL, id)
	w.Header().Set("Content-Type", "text/plain")
	if existed {
		w.WriteHeader(http.StatusConflict)
	} else {
		w.WriteHeader(http.StatusCreated)
	}
	_, _ = w.Write([]byte(shortURL))
}

func (h *Handler) GetHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "id is empty", http.StatusBadRequest)
		return
	}

	original, ok, err := h.store.Get(r.Context(), id)
	if err != nil {
		http.Error(w, "storage error", http.StatusInternalServerError)
		return
	}
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

	var in struct {
		URL string `json:"url"`
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil || strings.TrimSpace(in.URL) == "" {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	const maxAttempts = 5
	var id string
	for try := 0; try < maxAttempts; try++ {
		id = generateID()
		if err := h.store.Save(r.Context(), id, in.URL); err != nil {
			if errors.Is(err, model.ErrCollision) {
				continue
			}
			var dup *model.DuplicateURLError
			if errors.As(err, &dup) {
				shortURL, _ := url.JoinPath(h.baseURL, dup.ExistingID)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusConflict)
				_ = json.NewEncoder(w).Encode(map[string]string{"result": shortURL})
				return
			}
			http.Error(w, "storage error", http.StatusInternalServerError)
			return
		}
		shortURL, _ := url.JoinPath(h.baseURL, id)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{"result": shortURL})
		return
	}

	http.Error(w, "cannot allocate id", http.StatusInternalServerError)
}

func (h *Handler) PingHandler(w http.ResponseWriter, r *http.Request) {
	if err := h.store.Ping(r.Context()); err != nil {
		http.Error(w, "db is not configured", http.StatusInternalServerError)
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

		var id string
		const maxAttempts = 5
		for try := 0; try < maxAttempts; try++ {
			id = generateID()
			if err := h.store.Save(r.Context(), id, urlStr); err != nil {
				if errors.Is(err, model.ErrCollision) {
					continue
				}
				var dup *model.DuplicateURLError
				if errors.As(err, &dup) {
					id = dup.ExistingID
				} else {
					http.Error(w, "storage error", http.StatusInternalServerError)
					return
				}
			}
			shortURL, _ := url.JoinPath(h.baseURL, id)
			out = append(out, batchResItem{CorrelationID: it.CorrelationID, ShortURL: shortURL})
			break
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(out)
}

func (h *Handler) saveWithRetries(ctx context.Context, original string, maxTry int) (id string, existed bool, err error) {
	for i := 0; i < maxTry; i++ {
		id = generateID()
		err = h.store.Save(ctx, id, original)
		if err == nil {
			return id, false, nil
		}

		if errors.Is(err, model.ErrCollision) {
			continue
		}

		var dup *model.DuplicateURLError
		if errors.As(err, &dup) {
			return dup.ExistingID, true, nil
		}

		if errors.Is(err, model.ErrDuplicateOriginal) {
			if existID, ok, e := h.store.FindByOriginal(ctx, original); e == nil && ok {
				return existID, true, nil
			}
			return "", true, nil
		}

		return "", false, err
	}
	return "", false, model.ErrCollision
}
