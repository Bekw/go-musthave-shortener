package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
)

func hasToken(header, token string) bool {
	for _, p := range strings.Split(header, ",") {
		if strings.TrimSpace(p) == token {
			return true
		}
	}
	return false
}

type gzipReadCloser struct {
	gr *gzip.Reader
	rc io.ReadCloser
}

func (g gzipReadCloser) Read(p []byte) (int, error) { return g.gr.Read(p) }
func (g gzipReadCloser) Close() error {
	_ = g.gr.Close()
	return g.rc.Close()
}

type gzResponseWriter struct {
	http.ResponseWriter
	req         *http.Request
	gzw         *gzip.Writer
	wroteHeader bool
}

func (w *gzResponseWriter) shouldCompress() bool {
	if !hasToken(w.req.Header.Get("Accept-Encoding"), "gzip") {
		return false
	}
	ct := w.Header().Get("Content-Type")
	return strings.HasPrefix(ct, "application/json") ||
		strings.HasPrefix(ct, "text/html")
}

func (w *gzResponseWriter) startGzip() {
	if w.gzw != nil {
		return
	}
	if !w.shouldCompress() {
		return
	}
	w.Header().Del("Content-Length")
	w.Header().Set("Content-Encoding", "gzip")
	w.Header().Add("Vary", "Accept-Encoding")
	w.gzw = gzip.NewWriter(w.ResponseWriter)
}

func (w *gzResponseWriter) WriteHeader(status int) {
	w.startGzip()
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(status)
}

func (w *gzResponseWriter) Write(p []byte) (int, error) {
	if !w.wroteHeader {
		w.startGzip()
	}
	if w.gzw != nil {
		return w.gzw.Write(p)
	}
	return w.ResponseWriter.Write(p)
}

func (w *gzResponseWriter) Close() error {
	if w.gzw != nil {
		return w.gzw.Close()
	}
	return nil
}

func Gzip() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if hasToken(r.Header.Get("Content-Encoding"), "gzip") {
				if gr, err := gzip.NewReader(r.Body); err == nil {
					r.Body = gzipReadCloser{gr: gr, rc: r.Body}
				}
			}

			gzw := &gzResponseWriter{ResponseWriter: w, req: r}
			defer gzw.Close()

			next.ServeHTTP(gzw, r)
		})
	}
}
