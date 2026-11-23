// Package fetch downloads the package of a check and proves it is the one
// the check names.
package fetch

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// ErrMismatch means the bytes read are not the package the check names:
// another size or another SHA-256. Unlike a failed request, reading the
// link again gives the same bytes.
var ErrMismatch = errors.New("fetch: the package is not the one the check names")

// Package reads the package at link, which must be size bytes with the
// SHA-256 sum in lower-case hex. It never reads more than one byte past
// size.
func Package(ctx context.Context, client *http.Client, link string, size int64, sum string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
	if err != nil {
		return nil, fmt.Errorf("fetch: build the request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		// The link is presigned; the URL in the error would carry its signature.
		return nil, errors.New("fetch: the package could not be read")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch: the bucket answered %d", resp.StatusCode)
	}
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, io.LimitReader(resp.Body, size+1)); err != nil {
		return nil, errors.New("fetch: the package was cut off")
	}
	got := sha256.Sum256(buf.Bytes())
	switch {
	case int64(buf.Len()) != size:
		return nil, fmt.Errorf("%w: it has another size", ErrMismatch)
	case hex.EncodeToString(got[:]) != sum:
		return nil, fmt.Errorf("%w: it has another SHA-256", ErrMismatch)
	}
	return buf.Bytes(), nil
}
