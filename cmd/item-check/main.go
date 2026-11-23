// Command item-check runs one check of an item version for Plenigon
// Communities. A workflow run started by the service runs it with the id of
// the check: it asks GitHub for an OIDC token for the service, claims the
// check, downloads the package, proves it is the one the version lists,
// runs the checks, and reports their results.
//
// It reads:
//
//	CHECK_ID                        the check to run, the workflow's input
//	COMMUNITIES_URL                 the service's https URL, also the
//	                                token's audience
//	ACTIONS_ID_TOKEN_REQUEST_URL    set by GitHub Actions with the
//	ACTIONS_ID_TOKEN_REQUEST_TOKEN  id-token: write permission
//
// A failure that may pass on another run, such as a refused claim or a
// bucket that does not answer, exits 1 without a report, and the service
// dispatches the check again. A package that is not the one listed is
// reported as failed.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"syscall"
	"time"

	"github.com/plenigon/plenior-hub/internal/check"
	"github.com/plenigon/plenior-hub/internal/communities"
	"github.com/plenigon/plenior-hub/internal/fetch"
	"github.com/plenigon/plenior-hub/internal/oidc"
)

// requestTimeout bounds each request; a package of 100 MiB reads well
// within it on a hosted runner.
const requestTimeout = 5 * time.Minute

// checkIDPattern is a check id: a UUID, so it is safe in a path.
var checkIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// formats are the checks this hub runs, by the name a format registers.
var formats = check.Registry{}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := run(ctx, os.Getenv, &http.Client{Timeout: requestTimeout}, formats, os.Stdout); err != nil {
		logger.Error("item check failed", "error", err)
		os.Exit(1)
	}
}

// run runs the check its environment names, reading it with getenv, and
// writes each check's name and result to out. The run's log is public, so
// messages, which may name a private item, go to the service only.
func run(ctx context.Context, getenv func(string) string, client *http.Client, reg check.Registry, out io.Writer) error {
	id, base := getenv("CHECK_ID"), getenv("COMMUNITIES_URL")
	u, err := url.Parse(base)
	switch {
	case !checkIDPattern.MatchString(id):
		return errors.New("CHECK_ID is not a check id")
	case err != nil || u.Scheme != "https" || u.Host == "":
		return errors.New("COMMUNITIES_URL is not an https URL")
	}
	token, err := oidc.Token(ctx, client, getenv("ACTIONS_ID_TOKEN_REQUEST_URL"), getenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN"), base)
	if err != nil {
		return err
	}
	svc := &communities.Client{BaseURL: base, Token: token, HTTP: client}
	job, err := svc.Claim(ctx, id)
	if err != nil {
		return fmt.Errorf("claim the check: %w", err)
	}
	pkg, err := fetch.Package(ctx, client, job.Package.URL, job.Size, job.SHA256)
	if err != nil && !errors.Is(err, fetch.ErrMismatch) {
		return err
	}
	results := check.Run(ctx, job, pkg, err, reg)
	if err := svc.Report(ctx, id, results); err != nil {
		return fmt.Errorf("report the results: %w", err)
	}
	for _, r := range results {
		if _, err := fmt.Fprintf(out, "%s: %s\n", r.Name, r.Status); err != nil {
			return err
		}
	}
	return nil
}
