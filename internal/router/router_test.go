package router

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/vfswiss/vfswiss/internal/config"
)

func TestYandexVerificationFileServedAtSiteRoot(t *testing.T) {
	staticDir := t.TempDir()

	handler := New(
		&config.Config{StaticDir: staticDir},
		nil,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	request := httptest.NewRequest(http.MethodGet, "/yandex_50d24b5586b736f1.html", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	content, err := os.ReadFile("yandex_50d24b5586b736f1.html")
	if err != nil {
		t.Fatal(err)
	}
	if response.Body.String() != string(content) {
		t.Fatalf("body = %q, want %q", response.Body.String(), content)
	}
}
