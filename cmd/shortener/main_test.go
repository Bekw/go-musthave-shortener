package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPostHandler(t *testing.T) {
	body := strings.NewReader("https://practicum.yandex.kz/")
	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", "text/plain")

	w := httptest.NewRecorder()
	postHandler(w, req)

	res := w.Result()
	if res.StatusCode != http.StatusCreated {
		t.Errorf("Код ответа не совпадает с ожидаемым")
	}
}
func TestGetHandler(t *testing.T) {
	id := "testId"
	urlStore[id] = "https://practicum.yandex.kz/"

	req := httptest.NewRequest(http.MethodGet, "/"+id, nil)
	w := httptest.NewRecorder()
	getHandler(w, req)

	res := w.Result()
	if res.StatusCode != http.StatusTemporaryRedirect {
		t.Errorf("Код ответа не совпадает с ожидаемым")
	}
	if loc := res.Header.Get("Location"); loc != "https://practicum.yandex.kz/" {
		t.Errorf("Адрес не совпадает с ожидаемым")
	}
}
