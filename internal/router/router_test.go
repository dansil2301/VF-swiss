package router

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/vfswiss/vfswiss/internal/config"
)

func TestYandexVerificationFileServedAtSiteRoot(t *testing.T) {
	staticDir := t.TempDir()
	content := []byte("Verification: e27940cec41102c4")
	if err := os.WriteFile(filepath.Join(staticDir, "yandex_e27940cec41102c4.html"), content, 0600); err != nil {
		t.Fatal(err)
	}

	handler := New(
		&config.Config{StaticDir: staticDir},
		nil,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	request := httptest.NewRequest(http.MethodGet, "/yandex_e27940cec41102c4.html", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if response.Body.String() != string(content) {
		t.Fatalf("body = %q, want %q", response.Body.String(), content)
	}
}
