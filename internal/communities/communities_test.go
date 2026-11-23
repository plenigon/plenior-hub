package communities_test

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/plenigon/plenior-hub/internal/communities"
)

const checkID = "019a0000-0000-7000-8000-000000000c01"

const job = `{"id": "` + checkID + `", "item": "@ana/mural", "version": "1.0.0", "format": "plenior.app/1",
	"check": "plenior-check", "media_type": "application/zip", "sha256": "` + "5d1f" + `", "size": 18324,
	"manifest": {"format": "plenior.app/1"},
	"package": {"url": "https://bucket.example/items/pkg?signed", "expires_at": "2026-11-23T17:00:00Z"}}`

func serve(t *testing.T, h http.HandlerFunc) *communities.Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return &communities.Client{BaseURL: srv.URL, Token: "id-token", HTTP: srv.Client()}
}

func TestClaimReturnsTheJob(t *testing.T) {
	t.Parallel()
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/item-checks/"+checkID || r.Header.Get("Authorization") != "Bearer id-token" {
			http.Error(w, "unexpected", http.StatusTeapot)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(job))
	})
	got, err := c.Claim(t.Context(), checkID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Item != "@ana/mural" || got.Version != "1.0.0" || got.Check != "plenior-check" || got.Size != 18324 ||
		got.MediaType != "application/zip" || got.Package.URL != "https://bucket.example/items/pkg?signed" ||
		!got.Package.ExpiresAt.Equal(time.Date(2026, 11, 23, 17, 0, 0, 0, time.UTC)) || string(got.Manifest) != `{"format": "plenior.app/1"}` {
		t.Errorf("Claim = %+v", got)
	}
}

func TestReportSendsEveryResult(t *testing.T) {
	t.Parallel()
	var body string
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/item-checks/"+checkID+"/result" || r.Header.Get("Content-Type") != "application/json" {
			http.Error(w, "unexpected", http.StatusTeapot)
			return
		}
		raw, _ := io.ReadAll(r.Body)
		body = string(raw)
		w.WriteHeader(http.StatusNoContent)
	})
	err := c.Report(t.Context(), checkID, []communities.Result{
		{Name: "package", Status: communities.Passed},
		{Name: "plenior-check", Status: communities.Failed, Message: "item.json names another item."},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"checks":[{"name":"package","status":"passed","message":""},` +
		`{"name":"plenior-check","status":"failed","message":"item.json names another item."}]}`
	if strings.TrimSpace(body) != want {
		t.Errorf("body = %s\nwant %s", body, want)
	}
}

func TestErrorsCarryTheServiceCode(t *testing.T) {
	t.Parallel()
	c := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{
			"code": "check_not_running", "message": "The check is not running; it expired or its result is recorded.",
		}})
	})
	_, err := c.Claim(t.Context(), checkID)
	var apiErr *communities.Error
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusConflict || apiErr.Code != "check_not_running" {
		t.Fatalf("Claim error = %v, want the service's code", err)
	}
	if strings.Contains(err.Error(), "id-token") {
		t.Errorf("error %q holds the token", err)
	}
}

func TestClaimRefusesAnInsecureOrBadJob(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"an http package link": strings.Replace(job, "https://bucket", "http://bucket", 1),
		"another check":        strings.Replace(job, checkID, "019a0000-0000-7000-8000-000000000c02", 1),
		"a negative size":      strings.Replace(job, "18324", "-1", 1),
		"not json":             "{",
	} {
		c := serve(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) })
		if _, err := c.Claim(t.Context(), checkID); err == nil {
			t.Errorf("%s: Claim succeeded", name)
		}
	}
}
