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
)

var urlStore = make(map[string]string)

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

	shortURL := fmt.Sprintf("http://localhost:8080/%s", id)

	w.WriteHeader(http.StatusCreated)
	w.Header().Set("Content-Type", "text/plain")
	_, _ = w.Write([]byte(shortURL))
}

func getHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "only GET allowed", http.StatusBadRequest)
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/")
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
	r := chi.NewRouter()

	r.Post("/", postHandler)
	r.Get("/{id}", getHandler)

	fmt.Println("Server started at :8080")
	log.Fatal(http.ListenAndServe(":8080", r))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			postHandler(w, r)
		} else if r.Method == http.MethodGet && r.URL.Path != "/" {
			getHandler(w, r)
		} else {
			http.Error(w, "bad request", http.StatusBadRequest)
		}
	})

	fmt.Println("Server started at :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
