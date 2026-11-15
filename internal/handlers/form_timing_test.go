package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestValidFormDuration(t *testing.T) {
	h := &Handlers{}
	csrfToken := "csrf-test-token"
	tests := []struct {
		name        string
		createdAt   time.Time
		tamperToken bool
		wantValid   bool
	}{
		{name: "enough time elapsed", createdAt: time.Now().Add(-6 * time.Second), wantValid: true},
		{name: "submitted too quickly", createdAt: time.Now(), wantValid: false},
		{name: "form expired", createdAt: time.Now().Add(-25 * time.Hour), wantValid: false},
		{name: "invalid signature", createdAt: time.Now().Add(-6 * time.Second), tamperToken: true, wantValid: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			token := h.newFormStartedToken(csrfToken, test.createdAt)
			if test.tamperToken {
				token = "invalid.signature"
			}
			form := url.Values{"form_started_at": {token}}
			r := httptest.NewRequest(http.MethodPost, "/request", strings.NewReader(form.Encode()))
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			r.AddCookie(&http.Cookie{Name: "csrf_token", Value: csrfToken})

			if got := h.validFormDuration(r); got != test.wantValid {
				t.Fatalf("validFormDuration() = %v, want %v", got, test.wantValid)
			}
		})
	}
}
