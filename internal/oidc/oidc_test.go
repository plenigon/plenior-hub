package oidc_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/plenigon/plenior-hub/internal/oidc"
)

func TestTokenAsksForTheAudience(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer request-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.URL.Query().Get("audience") != "https://communities.example" || r.URL.Query().Get("kept") != "yes" {
			http.Error(w, "wrong audience", http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"value": "id-token"}`))
	}))
	defer srv.Close()

	got, err := oidc.Token(t.Context(), srv.Client(), srv.URL+"/token?kept=yes", "request-token", "https://communities.example")
	if err != nil || got != "id-token" {
		t.Fatalf("Token = %q, %v; want the id token", got, err)
	}
}

func TestTokenFailures(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct {
		status int
		body   string
		want   string
	}{
		"refused":  {http.StatusForbidden, `{"message": "no"}`, "403"},
		"no value": {http.StatusOK, `{}`, "no token"},
		"not json": {http.StatusOK, `token`, "decode"},
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(c.status)
			_, _ = w.Write([]byte(c.body))
		}))
		_, err := oidc.Token(t.Context(), srv.Client(), srv.URL, "request-token", "https://communities.example")
		srv.Close()
		if err == nil || !strings.Contains(err.Error(), c.want) || strings.Contains(err.Error(), "request-token") {
			t.Errorf("%s: Token error = %v, want one mentioning %q without the request token", name, err, c.want)
		}
	}
	if _, err := oidc.Token(t.Context(), http.DefaultClient, "", "request-token", "aud"); err == nil {
		t.Error("Token without a request URL succeeded; want the missing id-token permission reported")
	}
}
