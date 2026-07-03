package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	"golang.org/x/oauth2"

	"github.com/rudrankriyam/Google-Health-CLI/internal/config"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestTokenSourcePersistsRefreshedToken(t *testing.T) {
	tokenPath := configTokenPath(t)
	expired := &oauth2.Token{
		AccessToken:  "old-access-token",
		TokenType:    "Bearer",
		RefreshToken: "old-refresh-token",
		Expiry:       time.Now().Add(-time.Hour),
	}
	if err := SaveToken(expired); err != nil {
		t.Fatalf("SaveToken() error = %v", err)
	}

	var refreshCalls int
	httpClient := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		refreshCalls++
		if req.Method != http.MethodPost {
			t.Fatalf("refresh method = %s, want POST", req.Method)
		}
		if got := req.URL.String(); got != "https://oauth2.googleapis.com/token" {
			t.Fatalf("refresh URL = %s", got)
		}
		if err := req.ParseForm(); err != nil {
			t.Fatalf("ParseForm() error = %v", err)
		}
		wantForm := url.Values{
			"client_id":     {"client-id"},
			"client_secret": {"client-secret"},
			"grant_type":    {"refresh_token"},
			"refresh_token": {"old-refresh-token"},
		}
		for key, want := range wantForm {
			if got := req.Form[key]; len(got) != len(want) || got[0] != want[0] {
				t.Fatalf("form[%s] = %v, want %v", key, got, want)
			}
		}
		return jsonResponse(t, map[string]any{
			"access_token":  "new-access-token",
			"token_type":    "Bearer",
			"refresh_token": "new-refresh-token",
			"expires_in":    3600,
		}), nil
	})}

	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, httpClient)
	source, err := TokenSource(ctx, config.Config{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		RedirectURL:  config.DefaultRedirectURL,
	})
	if err != nil {
		t.Fatalf("TokenSource() error = %v", err)
	}

	token, err := source.Token()
	if err != nil {
		t.Fatalf("Token() error = %v", err)
	}
	if token.AccessToken != "new-access-token" {
		t.Fatalf("AccessToken = %q", token.AccessToken)
	}
	if token.RefreshToken != "new-refresh-token" {
		t.Fatalf("RefreshToken = %q", token.RefreshToken)
	}
	if refreshCalls != 1 {
		t.Fatalf("refreshCalls = %d, want 1", refreshCalls)
	}

	stored, err := LoadToken()
	if err != nil {
		t.Fatalf("LoadToken() error = %v", err)
	}
	if stored.AccessToken != "new-access-token" {
		t.Fatalf("stored AccessToken = %q", stored.AccessToken)
	}
	if stored.RefreshToken != "new-refresh-token" {
		t.Fatalf("stored RefreshToken = %q", stored.RefreshToken)
	}
	if !stored.Valid() {
		t.Fatalf("stored token is not valid: %#v", stored)
	}
	info, err := os.Stat(tokenPath)
	if err != nil {
		t.Fatalf("Stat(%q) error = %v", tokenPath, err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("token mode = %v, want 0600", got)
	}
}

func configTokenPath(t *testing.T) string {
	t.Helper()
	path := t.TempDir() + "/token.json"
	t.Setenv("GHEALTH_TOKEN_FILE", path)
	return path
}

func jsonResponse(t *testing.T, payload any) *http.Response {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(body)),
	}
}
