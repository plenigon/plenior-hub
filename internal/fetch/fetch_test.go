package fetch_test

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/plenigon/plenior-hub/internal/fetch"
)

func sum(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

func TestPackageReadsTheBytesTheJobNames(t *testing.T) {
	t.Parallel()
	content := []byte("the package")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(content) }))
	defer srv.Close()

	got, err := fetch.Package(t.Context(), srv.Client(), srv.URL, int64(len(content)), sum(content))
	if err != nil || string(got) != "the package" {
		t.Fatalf("Package = %q, %v", got, err)
	}

	for name, c := range map[string]struct {
		size int64
		hash string
	}{
		"a longer package":  {int64(len(content)) - 1, sum(content)},
		"a shorter package": {int64(len(content)) + 1, sum(content)},
		"another hash":      {int64(len(content)), sum([]byte("other"))},
	} {
		if _, err := fetch.Package(t.Context(), srv.Client(), srv.URL, c.size, c.hash); !errors.Is(err, fetch.ErrMismatch) {
			t.Errorf("%s: Package = %v, want ErrMismatch", name, err)
		}
	}
}

func TestPackageFailuresAreNotMismatches(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "expired", http.StatusForbidden)
	}))
	defer srv.Close()
	_, err := fetch.Package(t.Context(), srv.Client(), srv.URL, 1, sum([]byte("x")))
	if err == nil || errors.Is(err, fetch.ErrMismatch) {
		t.Errorf("Package of a refused link = %v, want an error that is no mismatch", err)
	}
}
