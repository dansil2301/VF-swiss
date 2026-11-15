package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"time"
)

type ctxKey string

const RequestIDKey ctxKey = "request_id"

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if !validRequestID(requestID) {
			requestID = newRequestID()
		}
		w.Header().Set("X-Request-ID", requestID)
		ctx := context.WithValue(r.Context(), RequestIDKey, requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func Logger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			srw := &statusResponseWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(srw, r)
			logger.Info("http request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", srw.status,
				"duration", time.Since(start).String(),
				"request_id", r.Context().Value(RequestIDKey),
			)
		})
	}
}

func Recovery(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					logger.Error("panic recovered", "error", rec)
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		w.Header().Set(
			"Content-Security-Policy",
			"default-src 'self'; "+
				"img-src 'self' data: https://yandex.ru https://*.yandex.ru; "+
				"style-src 'self' 'unsafe-inline' https://yandex.ru https://*.yandex.ru; "+
				"script-src 'self' https://yandex.ru https://*.yandex.ru https://challenges.cloudflare.com; "+
				"connect-src 'self' https://challenges.cloudflare.com; "+
				"frame-src 'self' https://yandex.ru https://*.yandex.ru https://challenges.cloudflare.com;",
		)

		next.ServeHTTP(w, r)
	})
}
func validRequestID(value string) bool {
	if value == "" {
		return false
	}
	if len(value) > 64 {
		return false
	}
	match, _ := regexp.MatchString(`^[A-Za-z0-9_-]+$`, value)
	return match
}

func newRequestID() string {
	b := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return time.Now().UTC().Format("20060102150405")
	}
	return hex.EncodeToString(b)
}

type statusResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusResponseWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}
