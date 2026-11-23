// Package oidc requests the OpenID Connect token GitHub Actions issues to a
// workflow run that has the id-token permission.
package oidc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// maxResponseBytes bounds the answer of the token endpoint.
const maxResponseBytes = 64 << 10

// Token returns a token for audience from the endpoint at requestURL, the
// ACTIONS_ID_TOKEN_REQUEST_URL of the run, authorized by requestToken, its
// ACTIONS_ID_TOKEN_REQUEST_TOKEN. Neither the request token nor the token
// is ever put in an error.
func Token(ctx context.Context, client *http.Client, requestURL, requestToken, audience string) (string, error) {
	if requestURL == "" || requestToken == "" {
		return "", errors.New("oidc: the run has no id-token permission")
	}
	u, err := url.Parse(requestURL)
	if err != nil {
		return "", errors.New("oidc: the token request URL is not valid")
	}
	q := u.Query()
	q.Set("audience", audience)
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", fmt.Errorf("oidc: build the request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+requestToken)
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("oidc: request a token: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("oidc: the token endpoint answered %d", resp.StatusCode)
	}
	var body struct {
		Value string `json:"value"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(&body); err != nil {
		return "", fmt.Errorf("oidc: decode the token: %w", err)
	}
	if body.Value == "" {
		return "", errors.New("oidc: the endpoint answered no token")
	}
	return body.Value, nil
}
