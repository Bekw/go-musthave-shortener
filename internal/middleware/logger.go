package middleware

import (
	"net/http"
	"time"

	"go.uber.org/zap"
)

type respWriter struct {
	http.ResponseWriter
	status int
	size   int
}

func (w *respWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *respWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(b)
	w.size += n
	return n, err
}

func Logger(l *zap.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			wrapped := &respWriter{ResponseWriter: w}

			next.ServeHTTP(wrapped, r)

			l.Info("http",
				zap.String("method", r.Method),
				zap.String("uri", r.RequestURI),
				zap.Int("status", wrapped.status),
				zap.Int("size", wrapped.size),
				zap.Duration("duration", time.Since(start)),
			)
		})
	}
}
