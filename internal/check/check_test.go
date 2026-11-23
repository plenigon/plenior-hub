package check_test

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/plenigon/plenior-hub/internal/check"
	"github.com/plenigon/plenior-hub/internal/communities"
	"github.com/plenigon/plenior-hub/internal/fetch"
)

func zipped(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, err := w.Create("item.json")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.Write([]byte(`{}`))
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func gzipped(t *testing.T, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	_, _ = w.Write(content)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func results(rs []communities.Result) string {
	var out []string
	for _, r := range rs {
		out = append(out, r.Name+"="+r.Status)
	}
	return strings.Join(out, " ")
}

func job(mediaType, name string) communities.Job {
	return communities.Job{Item: "@ana/mural", Version: "1.0.0", Format: "plenior.app/1", Check: name, MediaType: mediaType}
}

func TestThePackageCheckReadsTheMediaType(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct {
		mediaType string
		pkg       []byte
		want      string
	}{
		"a zip":              {"application/zip", zipped(t), "passed"},
		"a broken zip":       {"application/zip", []byte("PK not a zip"), "failed"},
		"a gzip":             {"application/gzip", gzipped(t, []byte("tar bytes")), "passed"},
		"a cut gzip":         {"application/gzip", gzipped(t, bytes.Repeat([]byte("a"), 4096))[:20], "failed"},
		"a gzip that is not": {"application/gzip", []byte("plain"), "failed"},
		"json":               {"application/json", []byte(`{"steps": []}`), "passed"},
		"broken json":        {"application/json", []byte(`{"steps": `), "failed"},
		"another type":       {"application/octet-stream", []byte{1, 2, 3}, "passed"},
	} {
		got := check.Run(t.Context(), job(c.mediaType, "package"), c.pkg, nil, nil)
		if results(got) != "package="+c.want {
			t.Errorf("%s: Run = %+v, want the package check %s", name, got, c.want)
		}
	}
}

func TestAFormatCheckRunsAfterThePackageCheck(t *testing.T) {
	t.Parallel()
	var seen communities.Job
	reg := check.Registry{
		"plenior-check": func(_ context.Context, j communities.Job, pkg []byte) (string, string) {
			seen = j
			if len(pkg) == 0 {
				return communities.Failed, "empty"
			}
			return communities.Passed, ""
		},
	}
	got := check.Run(t.Context(), job("application/zip", "plenior-check"), zipped(t), nil, reg)
	if results(got) != "package=passed plenior-check=passed" || seen.Item != "@ana/mural" {
		t.Errorf("Run = %+v, seen %+v", got, seen)
	}

	// A package that fails its own check is not given to the format's.
	seen = communities.Job{}
	got = check.Run(t.Context(), job("application/zip", "plenior-check"), []byte("PK broken"), nil, reg)
	if results(got) != "package=failed plenior-check=failed" || seen.Item != "" {
		t.Errorf("Run of a broken package = %+v, seen %+v", got, seen)
	}
	if !strings.Contains(got[1].Message, "package check failed") {
		t.Errorf("message = %q", got[1].Message)
	}
}

func TestAMismatchedPackageFailsEveryCheck(t *testing.T) {
	t.Parallel()
	err := fmt.Errorf("%w: it has another SHA-256", fetch.ErrMismatch)
	got := check.Run(t.Context(), job("application/zip", "plenior-check"), nil, err, check.Registry{})
	if results(got) != "package=failed plenior-check=failed" || !strings.Contains(got[0].Message, "SHA-256") {
		t.Errorf("Run = %+v", got)
	}
}

func TestAnUnknownCheckFails(t *testing.T) {
	t.Parallel()
	got := check.Run(t.Context(), job("application/zip", "other-check"), zipped(t), nil, check.Registry{})
	if results(got) != "package=passed other-check=failed" || !strings.Contains(got[1].Message, "other-check") {
		t.Errorf("Run = %+v, want the unknown check failed", got)
	}
}

func TestAPanickingCheckFails(t *testing.T) {
	t.Parallel()
	reg := check.Registry{"plenior-check": func(context.Context, communities.Job, []byte) (string, string) { panic("boom") }}
	got := check.Run(t.Context(), job("application/zip", "plenior-check"), zipped(t), nil, reg)
	if results(got) != "package=passed plenior-check=failed" {
		t.Errorf("Run = %+v, want the check failed", got)
	}
}

func TestMessagesFitTheService(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("é", check.MaxMessageBytes)
	reg := check.Registry{
		"plenior-check": func(context.Context, communities.Job, []byte) (string, string) { return communities.Failed, long },
		"bad-status":    func(context.Context, communities.Job, []byte) (string, string) { return "skipped", "" },
	}
	got := check.Run(t.Context(), job("application/zip", "plenior-check"), zipped(t), nil, reg)
	if m := got[1].Message; len(m) > check.MaxMessageBytes || !utf8.ValidString(m) {
		t.Errorf("message of %d bytes, valid %v", len(m), utf8.ValidString(m))
	}
	// The format's own check passes or fails; a skip is a failure.
	got = check.Run(t.Context(), job("application/zip", "bad-status"), zipped(t), nil, reg)
	if results(got) != "package=passed bad-status=failed" {
		t.Errorf("Run = %+v, want the skipped format check failed", got)
	}
}
