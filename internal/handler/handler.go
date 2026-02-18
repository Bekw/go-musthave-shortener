package handler

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/Bekw/go-musthave-shortener/internal/audit"
	"github.com/Bekw/go-musthave-shortener/internal/model"
	"github.com/Bekw/go-musthave-shortener/internal/service"
)

const userCookieName = "user_id"

// Handler provides HTTP handlers for the URL shortener service.
type Handler struct {
	store      model.Store         // Storage backend (in-memory, file, or PostgreSQL)
	urlService *service.URLService // Business logic service for URL operations
	baseURL    string              // Base URL for generating short links (e.g., http://localhost:8080)
	log        *zap.Logger         // Structured logger for request/error logging
	secretKey  []byte              // HMAC secret key for signing user cookies
	aud        *audit.Auditor      // Event auditor for tracking user actions
}

type userURL struct {
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
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

// NewHandler constructs a Handler with the given storage and base URL.
func NewHandler(store model.Store, baseURL string, log *zap.Logger) *Handler {
	if log == nil {
		log = zap.NewNop()
	}

	h := &Handler{
		store:      store,
		urlService: service.NewURLService(store, log),
		baseURL:    baseURL,
		log:        log,
		secretKey:  []byte("very-secret-key"),
	}

	return h
}

// newUserID generates a new unique user identifier.
func (h *Handler) newUserID() string {
	return service.GenerateID() + service.GenerateID()
}

// signUserID creates a signed cookie value for the user ID using HMAC-SHA256.
// Format: base64(userID:hex(hmac_signature))
func (h *Handler) signUserID(id string) string {
	mac := hmac.New(sha256.New, h.secretKey)
	mac.Write([]byte(id))
	sig := mac.Sum(nil)

	hexSig := make([]byte, hex.EncodedLen(len(sig)))
	hex.Encode(hexSig, sig)

	payload := make([]byte, len(id)+1+len(hexSig))
	copy(payload, id)
	payload[len(id)] = ':'
	copy(payload[len(id)+1:], hexSig)

	out := make([]byte, base64.URLEncoding.EncodedLen(len(payload)))
	base64.URLEncoding.Encode(out, payload)
	return string(out)
}

// parseUserID verifies and extracts the user ID from a signed cookie value.
// Returns the user ID and true if valid, empty string and false otherwise.
func (h *Handler) parseUserID(value string) (string, bool) {
	data, err := base64.URLEncoding.DecodeString(value)
	if err != nil {
		return "", false
	}

	i := bytes.IndexByte(data, ':')
	if i <= 0 || i >= len(data)-1 {
		return "", false
	}

	idBytes := data[:i]
	sigHex := data[i+1:]

	sigBytes := make([]byte, hex.DecodedLen(len(sigHex)))
	n, err := hex.Decode(sigBytes, sigHex)
	if err != nil {
		return "", false
	}
	sigBytes = sigBytes[:n]

	mac := hmac.New(sha256.New, h.secretKey)
	mac.Write(idBytes)
	expected := mac.Sum(nil)

	if !hmac.Equal(sigBytes, expected) {
		return "", false
	}

	return string(idBytes), true
}

// readUserID reads and validates the user ID from the request cookie.
// Returns (userID, hasCookie, isValid).
func (h *Handler) readUserID(r *http.Request) (string, bool, bool) {
	c, err := r.Cookie(userCookieName)
	if err != nil {
		if err == http.ErrNoCookie {
			return "", false, false
		}
		return "", false, false
	}

	id, ok := h.parseUserID(c.Value)
	if !ok || id == "" {
		return "", true, false
	}

	return id, true, true
}

// setUserCookie sets the signed user ID cookie in the response.
func (h *Handler) setUserCookie(w http.ResponseWriter, id string) {
	val := h.signUserID(id)
	http.SetCookie(w, &http.Cookie{
		Name:     userCookieName,
		Value:    val,
		Path:     "/",
		HttpOnly: true,
	})
}

// ensureUserID retrieves or creates a user ID, setting the cookie if needed.
func (h *Handler) ensureUserID(w http.ResponseWriter, r *http.Request) string {
	id, hasCookie, valid := h.readUserID(r)
	if !hasCookie || !valid || id == "" {
		id = h.newUserID()
	}
	if !hasCookie || !valid {
		h.setUserCookie(w, id)
	}
	return id
}

// PostHandler handles POST / with text/plain body (original URL) and returns a short URL.
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

	userID := h.ensureUserID(w, r)

	id, existed, err := h.urlService.SaveWithRetries(r.Context(), original, 5)
	if err != nil {
		h.log.Error("store save error", zap.Error(err))
		http.Error(w, "storage error", http.StatusInternalServerError)
		return
	}

	shortURL := h.baseURL + "/" + id

	if err := h.urlService.AddUserURL(r.Context(), userID, id); err != nil {
		h.log.Error("add user url", zap.Error(err))
		http.Error(w, "storage error", http.StatusInternalServerError)
		return
	}

	h.publish(r.Context(), audit.Event{
		TS:     time.Now().Unix(),
		Action: audit.ActionShorten,
		UserID: userID,
		URL:    original,
	})

	w.Header().Set("Content-Type", "text/plain")
	if existed {
		w.WriteHeader(http.StatusConflict)
	} else {
		w.WriteHeader(http.StatusCreated)
	}
	_, _ = w.Write([]byte(shortURL))
}

// GetHandler handles GET /{id} and redirects to the original URL.
func (h *Handler) GetHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "id is empty", http.StatusBadRequest)
		return
	}

	original, ok, err := h.urlService.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, model.ErrDeleted) {
			w.WriteHeader(http.StatusGone)
			return
		}
		http.Error(w, "storage error", http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "id not found", http.StatusBadRequest)
		return
	}

	uid := ""
	if userID, has, valid := h.readUserID(r); has && valid {
		uid = userID
	}

	h.publish(r.Context(), audit.Event{
		TS:     time.Now().Unix(),
		Action: audit.ActionFollow,
		UserID: uid,
		URL:    original,
	})

	w.Header().Set("Location", original)
	w.WriteHeader(http.StatusTemporaryRedirect)
}

// PostJSONHandler handles POST /api/shorten with JSON body and returns JSON response.
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
	if err := dec.Decode(&in); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	original := strings.TrimSpace(in.URL)
	if original == "" {
		http.Error(w, "empty url", http.StatusBadRequest)
		return
	}

	userID := h.ensureUserID(w, r)

	id, existed, err := h.urlService.SaveWithRetries(r.Context(), original, 5)
	if err != nil {
		h.log.Error("store save error", zap.Error(err))
		http.Error(w, "storage error", http.StatusInternalServerError)
		return
	}

	shortURL := h.baseURL + "/" + id

	if err := h.urlService.AddUserURL(r.Context(), userID, id); err != nil {
		h.log.Error("add user url", zap.Error(err))
		http.Error(w, "storage error", http.StatusInternalServerError)
		return
	}

	h.publish(r.Context(), audit.Event{
		TS:     time.Now().Unix(),
		Action: audit.ActionShorten,
		UserID: userID,
		URL:    original,
	})

	w.Header().Set("Content-Type", "application/json")
	if existed {
		w.WriteHeader(http.StatusConflict)
	} else {
		w.WriteHeader(http.StatusCreated)
	}

	_ = json.NewEncoder(w).Encode(shortenResponse{Result: shortURL})
}

// PingHandler handles GET /ping for database health checks.
func (h *Handler) PingHandler(w http.ResponseWriter, r *http.Request) {
	if err := h.store.Ping(r.Context()); err != nil {
		http.Error(w, "db is not configured", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// PostBatchHandler handles POST /api/shorten/batch for batch URL shortening.
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

	userID := h.ensureUserID(w, r)

	out := make([]batchResItem, 0, len(in))

	for _, it := range in {
		urlStr := strings.TrimSpace(it.OriginalURL)
		if urlStr == "" || it.CorrelationID == "" {
			http.Error(w, "empty url or correlation_id", http.StatusBadRequest)
			return
		}

		id, _, err := h.urlService.SaveWithRetries(r.Context(), urlStr, 5)
		if err != nil {
			http.Error(w, "storage error", http.StatusInternalServerError)
			return
		}

		shortURL := h.baseURL + "/" + id

		if err := h.urlService.AddUserURL(r.Context(), userID, id); err != nil {
			h.log.Error("add user url", zap.Error(err))
			http.Error(w, "storage error", http.StatusInternalServerError)
			return
		}

		out = append(out, batchResItem{
			CorrelationID: it.CorrelationID,
			ShortURL:      shortURL,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(out)
}

// GetUserURLsHandler handles GET /api/user/urls to retrieve all URLs for the current user.
func (h *Handler) GetUserURLsHandler(w http.ResponseWriter, r *http.Request) {
	userID, hasCookie, valid := h.readUserID(r)

	if hasCookie && !valid {
		http.Error(w, "invalid user cookie", http.StatusUnauthorized)
		return
	}

	if !hasCookie || userID == "" {
		userID = h.newUserID()
		h.setUserCookie(w, userID)
		w.WriteHeader(http.StatusNoContent)
		return
	}

	items, err := h.urlService.GetUserURLs(r.Context(), userID)
	if err != nil {
		h.log.Error("get user urls error", zap.Error(err))
		http.Error(w, "storage error", http.StatusInternalServerError)
		return
	}

	if len(items) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	type respItem struct {
		ShortURL    string `json:"short_url"`
		OriginalURL string `json:"original_url"`
	}

	out := make([]respItem, 0, len(items))
	for _, it := range items {
		short := h.baseURL + "/" + it.ID
		out = append(out, respItem{
			ShortURL:    short,
			OriginalURL: it.OriginalURL,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(out)
}

// DeleteUserURLsHandler handles DELETE /api/user/urls for async deletion of user URLs.
func (h *Handler) DeleteUserURLsHandler(w http.ResponseWriter, r *http.Request) {
	userID, hasCookie, valid := h.readUserID(r)
	if hasCookie && !valid {
		http.Error(w, "invalid user cookie", http.StatusUnauthorized)
		return
	}

	if !hasCookie || userID == "" {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		http.Error(w, "Content-Type must be application/json", http.StatusBadRequest)
		return
	}

	var ids []string
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&ids); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	if len(ids) == 0 {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	if err := h.urlService.ScheduleDelete(userID, ids); err != nil {
		h.log.Warn("failed to schedule delete",
			zap.String("userID", userID),
			zap.Int("ids_count", len(ids)),
			zap.Error(err),
		)
	}

	h.publish(r.Context(), audit.Event{
		TS:     time.Now().Unix(),
		Action: audit.ActionDelete,
		UserID: userID,
		URL:    strings.Join(ids, ","),
	})

	w.WriteHeader(http.StatusAccepted)
}

// SetAuditor sets the auditor for tracking events.
func (h *Handler) SetAuditor(a *audit.Auditor) {
	h.aud = a
}

// publish sends an audit event if an auditor is configured.
func (h *Handler) publish(ctx context.Context, e audit.Event) {
	if h.aud == nil {
		return
	}
	h.aud.Publish(ctx, e)
}
// Shutdown gracefully stops background workers.
func (h *Handler) Shutdown(ctx context.Context) error {
	if h.urlService == nil {
		return nil
	}
	return h.urlService.Shutdown(ctx)
}
