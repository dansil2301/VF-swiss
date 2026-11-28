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

func TestYandexVerificationFilesServedAtSiteRoot(t *testing.T) {
	staticDir := t.TempDir()
	handler := New(
		&config.Config{StaticDir: staticDir},
		nil,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	for _, file := range []string{
		"yandex_e27940cec41102c4.html",
		"yandex_50d24b5586b736f1.html",
	} {
		t.Run(file, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/"+file, nil)
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
			}
			content, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			if response.Body.String() != string(content) {
				t.Fatalf("body = %q, want %q", response.Body.String(), content)
			}
		})
	}
}
