package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/plenigon/plenior-hub/internal/check"
	"github.com/plenigon/plenior-hub/internal/communities"
)

const checkID = "019a0000-0000-7000-8000-000000000c01"

// service fakes GitHub's token endpoint, the Communities check endpoints,
// and the bucket, on one TLS server.
type service struct {
	mu       sync.Mutex
	pkg      []byte
	listed   []byte
	bucket   int
	claim    int
	reported []communities.Result
}

func (s *service) handler(t *testing.T, srv **httptest.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		switch {
		case r.URL.Path == "/token":
			if r.Header.Get("Authorization") != "Bearer request-token" || r.URL.Query().Get("audience") != (*srv).URL {
				http.Error(w, "no", http.StatusForbidden)
				return
			}
			_, _ = w.Write([]byte(`{"value": "id-token"}`))
		case r.Header.Get("Authorization") != "Bearer id-token" && strings.HasPrefix(r.URL.Path, "/v1/"):
			http.Error(w, "no", http.StatusUnauthorized)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/item-checks/"+checkID:
			if s.claim != 0 {
				w.WriteHeader(s.claim)
				_, _ = w.Write([]byte(`{"error": {"code": "check_not_running", "message": "The check is not running."}}`))
				return
			}
			sum := sha256.Sum256(s.listed)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": checkID, "item": "@ana/mural", "version": "1.0.0", "format": "plenior.app/1", "check": "plenior-check",
				"media_type": "application/zip", "sha256": hex.EncodeToString(sum[:]), "size": len(s.listed),
				"manifest": map[string]string{"format": "plenior.app/1"},
				"package":  map[string]any{"url": (*srv).URL + "/pkg?signed", "expires_at": time.Now().Add(10 * time.Minute)},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/item-checks/"+checkID+"/result":
			var body struct {
				Checks []communities.Result `json:"checks"`
			}
			raw, _ := io.ReadAll(r.Body)
			if err := json.Unmarshal(raw, &body); err != nil {
				t.Errorf("result body %s: %v", raw, err)
			}
			s.reported = body.Checks
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Path == "/pkg":
			if s.bucket != 0 {
				w.WriteHeader(s.bucket)
				return
			}
			_, _ = w.Write(s.pkg)
		default:
			http.NotFound(w, r)
		}
	}
}

func zipped(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	if _, err := w.Create("item.json"); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func start(t *testing.T, s *service) (func(string) string, *http.Client) {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewTLSServer(s.handler(t, &srv))
	t.Cleanup(srv.Close)
	env := map[string]string{
		"CHECK_ID":                       checkID,
		"COMMUNITIES_URL":                srv.URL,
		"ACTIONS_ID_TOKEN_REQUEST_URL":   srv.URL + "/token",
		"ACTIONS_ID_TOKEN_REQUEST_TOKEN": "request-token",
	}
	return func(k string) string { return env[k] }, srv.Client()
}

var passing = check.Registry{
	"plenior-check": func(_ context.Context, j communities.Job, _ []byte) (string, string) {
		return communities.Passed, "checked " + j.Item
	},
}

func reported(rs []communities.Result) string {
	var out []string
	for _, r := range rs {
		out = append(out, r.Name+"="+r.Status)
	}
	return strings.Join(out, " ")
}

func TestARunClaimsChecksAndReports(t *testing.T) {
	t.Parallel()
	pkg := zipped(t)
	s := &service{pkg: pkg, listed: pkg}
	env, client := start(t, s)
	if err := run(t.Context(), env, client, passing, io.Discard); err != nil {
		t.Fatal(err)
	}
	if reported(s.reported) != "package=passed plenior-check=passed" || s.reported[1].Message != "checked @ana/mural" {
		t.Errorf("reported %+v", s.reported)
	}
}

// The log of a public repository's run is public, so it names each check
// and its result only: messages may name a private item and go to the
// service alone.
func TestTheLogShowsOnlyResults(t *testing.T) {
	t.Parallel()
	pkg := zipped(t)
	s := &service{pkg: pkg, listed: pkg}
	env, client := start(t, s)
	var out bytes.Buffer
	if err := run(t.Context(), env, client, passing, &out); err != nil {
		t.Fatal(err)
	}
	if got, want := out.String(), "package: passed\nplenior-check: passed\n"; got != want {
		t.Errorf("log = %q, want %q", got, want)
	}
}

func TestAnotherPackageIsReportedAsFailed(t *testing.T) {
	t.Parallel()
	s := &service{pkg: zipped(t), listed: []byte("what the version listed")}
	env, client := start(t, s)
	if err := run(t.Context(), env, client, passing, io.Discard); err != nil {
		t.Fatal(err)
	}
	if reported(s.reported) != "package=failed plenior-check=failed" {
		t.Errorf("reported %+v", s.reported)
	}
}

func TestFailuresThatMayPassLaterAreNotReported(t *testing.T) {
	t.Parallel()
	for name, s := range map[string]*service{
		"a refused claim":  {claim: http.StatusConflict},
		"a failing bucket": {bucket: http.StatusInternalServerError, pkg: []byte("x"), listed: []byte("x")},
		"an expired link":  {bucket: http.StatusForbidden, pkg: []byte("x"), listed: []byte("x")},
	} {
		env, client := start(t, s)
		err := run(t.Context(), env, client, passing, io.Discard)
		if err == nil || s.reported != nil || strings.Contains(err.Error(), "id-token") {
			t.Errorf("%s: run = %v, reported %+v; want an error and no report", name, err, s.reported)
		}
	}
}

func TestRunRefusesABadEnvironment(t *testing.T) {
	t.Parallel()
	s := &service{}
	env, client := start(t, s)
	for name, bad := range map[string]map[string]string{
		"no check id":      {"CHECK_ID": ""},
		"a bad check id":   {"CHECK_ID": "../admin"},
		"an http service":  {"COMMUNITIES_URL": "http://communities.example"},
		"no token request": {"ACTIONS_ID_TOKEN_REQUEST_URL": ""},
	} {
		lookup := func(k string) string {
			if v, ok := bad[k]; ok {
				return v
			}
			return env(k)
		}
		if err := run(t.Context(), lookup, client, passing, io.Discard); err == nil {
			t.Errorf("%s: run succeeded", name)
		}
	}
	if s.claim != 0 || s.reported != nil {
		t.Error("a bad environment reached the service")
	}
}
