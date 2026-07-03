package config

import (
	"os"
	"path/filepath"
	"testing"
)

func setConfigDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("GHEALTH_CONFIG_DIR", dir)
	// Make sure environment overrides do not leak into assertions.
	for _, key := range []string{
		"GHEALTH_BASE_URL", "GHEALTH_USER", "GHEALTH_PROJECT",
		"GHEALTH_CLIENT_ID", "GHEALTH_CLIENT_SECRET", "GHEALTH_REDIRECT_URI", "GHEALTH_SCOPES",
	} {
		t.Setenv(key, "")
	}
	return dir
}

func TestSaveLoadRoundTrip(t *testing.T) {
	setConfigDir(t)

	saved := Config{
		BaseURL:      "https://example.com/api/",
		User:         "/users/someone/",
		Project:      "my-project",
		ClientID:     " client-id ",
		ClientSecret: "client-secret",
		RedirectURL:  "http://127.0.0.1:9999/callback",
		Scopes:       []string{"scope-a", "scope-b"},
	}
	if err := Save(saved); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.BaseURL != "https://example.com/api" {
		t.Fatalf("BaseURL = %q", loaded.BaseURL)
	}
	if loaded.User != "users/someone" {
		t.Fatalf("User = %q", loaded.User)
	}
	if loaded.Project != "my-project" {
		t.Fatalf("Project = %q", loaded.Project)
	}
	if loaded.ClientID != "client-id" {
		t.Fatalf("ClientID = %q", loaded.ClientID)
	}
	if loaded.ClientSecret != "client-secret" {
		t.Fatalf("ClientSecret = %q", loaded.ClientSecret)
	}
	if loaded.RedirectURL != "http://127.0.0.1:9999/callback" {
		t.Fatalf("RedirectURL = %q", loaded.RedirectURL)
	}
	if len(loaded.Scopes) != 2 || loaded.Scopes[0] != "scope-a" || loaded.Scopes[1] != "scope-b" {
		t.Fatalf("Scopes = %#v", loaded.Scopes)
	}
}

func TestLoadMissingFileUsesDefaults(t *testing.T) {
	setConfigDir(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.BaseURL != DefaultBaseURL {
		t.Fatalf("BaseURL = %q, want %q", cfg.BaseURL, DefaultBaseURL)
	}
	if cfg.User != DefaultUser {
		t.Fatalf("User = %q, want %q", cfg.User, DefaultUser)
	}
	if cfg.RedirectURL != DefaultRedirectURL {
		t.Fatalf("RedirectURL = %q, want %q", cfg.RedirectURL, DefaultRedirectURL)
	}
}

func TestLoadCorruptFileReturnsError(t *testing.T) {
	dir := setConfigDir(t)
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(); err == nil {
		t.Fatal("Load of corrupt config: expected error")
	}
}
