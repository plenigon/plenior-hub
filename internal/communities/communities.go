// Package communities is the client of the item check endpoints of
// Plenigon Communities (its ADR 018): a run claims a check, which tells it
// what to check, and reports the result of every check it made.
package communities

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Statuses of a check.
const (
	Passed  = "passed"
	Failed  = "failed"
	Skipped = "skipped"
)

// maxResponseBytes bounds an answer of the service; a job holds a
// manifest of at most 64 KiB.
const maxResponseBytes = 256 << 10

// Job is what a claimed check asks the run to check.
type Job struct {
	ID        string          `json:"id"`
	Item      string          `json:"item"`
	Version   string          `json:"version"`
	Format    string          `json:"format"`
	Check     string          `json:"check"`
	MediaType string          `json:"media_type"`
	SHA256    string          `json:"sha256"`
	Size      int64           `json:"size"`
	Manifest  json.RawMessage `json:"manifest"`
	Package   Package         `json:"package"`
}

// Package is where the run reads the version's package, until ExpiresAt.
type Package struct {
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Result is the outcome of one check.
type Result struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

// Error is an answer of the service that is not a success.
type Error struct {
	Status  int
	Code    string
	Message string
}

func (e *Error) Error() string {
	return fmt.Sprintf("communities: answered %d %s: %s", e.Status, e.Code, e.Message)
}

// Client calls the service at BaseURL with the run's OIDC Token.
type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

// Claim claims the check id and returns its job.
func (c *Client) Claim(ctx context.Context, id string) (Job, error) {
	resp, err := c.do(ctx, http.MethodGet, "/v1/item-checks/"+url.PathEscape(id), nil)
	if err != nil {
		return Job{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	var job Job
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(&job); err != nil {
		return Job{}, fmt.Errorf("communities: decode the job: %w", err)
	}
	link, err := url.Parse(job.Package.URL)
	switch {
	case job.ID != id:
		return Job{}, fmt.Errorf("communities: claimed check %s, got the job of %q", id, job.ID)
	case job.Size < 0:
		return Job{}, errors.New("communities: the job has a negative size")
	case err != nil || link.Scheme != "https" || link.Host == "":
		return Job{}, errors.New("communities: the package link is not an https URL")
	}
	return job, nil
}

// Report records results as the outcome of the check id.
func (c *Client) Report(ctx context.Context, id string, results []Result) error {
	body, err := json.Marshal(struct {
		Checks []Result `json:"checks"`
	}{results})
	if err != nil {
		return fmt.Errorf("communities: encode the results: %w", err)
	}
	resp, err := c.do(ctx, http.MethodPost, "/v1/item-checks/"+url.PathEscape(id)+"/result", body)
	if err != nil {
		return err
	}
	return resp.Body.Close()
}

// do sends a request and returns a successful answer, or the service's
// error. The token is never put in an error.
func (c *Client) do(ctx context.Context, method, path string, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimSuffix(c.BaseURL, "/")+path, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("communities: build the request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("communities: %s %s: %w", method, path, err)
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return resp, nil
	}
	defer func() { _ = resp.Body.Close() }()
	var e struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(&e)
	return nil, &Error{Status: resp.StatusCode, Code: e.Error.Code, Message: e.Error.Message}
}
