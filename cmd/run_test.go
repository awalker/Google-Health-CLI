package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunTypesList(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunWithWriters([]string{"types", "list"}, "test", &stdout, &stderr)
	if code != ExitSuccess {
		t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("heart-rate-variability")) {
		t.Fatalf("types output missing HRV: %s", stdout.String())
	}
}

func TestRunAgentManifestIsJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunWithWriters([]string{"agent", "manifest"}, "test", &stdout, &stderr)
	if code != ExitSuccess {
		t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"name": "ghealth"`)) {
		t.Fatalf("manifest output = %s", stdout.String())
	}
}

func TestUnknownCommandIsUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunWithWriters([]string{"wat"}, "test", &stdout, &stderr)
	if code != ExitUsage {
		t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
	}
}

func TestAPICallWithoutTokenReturnsJSONAuthError(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	var stdout, stderr bytes.Buffer
	code := RunWithWriters([]string{
		"data", "list", "steps",
		"--from", "2026-05-08T00:00:00Z",
		"--to", "2026-05-09T00:00:00Z",
	}, "test", &stdout, &stderr)
	if code != ExitAuth {
		t.Fatalf("exit = %d, stdout = %s, stderr = %s", code, stdout.String(), stderr.String())
	}

	var payload map[string]string
	if err := json.Unmarshal(stderr.Bytes(), &payload); err != nil {
		t.Fatalf("stderr is not JSON: %v\n%s", err, stderr.String())
	}
	if payload["status"] != "error" {
		t.Fatalf("status = %q, payload = %#v", payload["status"], payload)
	}
	if payload["message"] == "" || payload["message"] == "unknown error" {
		t.Fatalf("message = %q, payload = %#v", payload["message"], payload)
	}
}

func TestCivilDateTimeValid(t *testing.T) {
	value, err := civilDateTime("2026-05-08")
	if err != nil {
		t.Fatalf("civilDateTime: %v", err)
	}
	date, ok := value["date"].(map[string]any)
	if !ok {
		t.Fatalf("date missing: %#v", value)
	}
	if date["year"] != 2026 || date["month"] != 5 || date["day"] != 8 {
		t.Fatalf("date = %#v", date)
	}
	if _, hasTime := value["time"]; hasTime {
		t.Fatalf("unexpected time component: %#v", value)
	}

	value, err = civilDateTime("2026-12-31T23:59:07Z")
	if err != nil {
		t.Fatalf("civilDateTime with time: %v", err)
	}
	clock, ok := value["time"].(map[string]any)
	if !ok {
		t.Fatalf("time missing: %#v", value)
	}
	if clock["hours"] != 23 || clock["minutes"] != 59 || clock["seconds"] != 7 {
		t.Fatalf("time = %#v", clock)
	}
}

func TestCivilDateTimeInvalid(t *testing.T) {
	cases := []struct {
		input   string
		wantErr string
	}{
		{"2026-13-01", "invalid month"},
		{"2026-00-01", "invalid month"},
		{"2026-01-32", "invalid day"},
		{"2026-01-00", "invalid day"},
		{"2026-01", "expected YYYY-MM-DD"},
		{"2026-01-02T25", "expected time"},
		{"2026-0a-01", "invalid number"},
	}
	for _, tc := range cases {
		_, err := civilDateTime(tc.input)
		if err == nil {
			t.Fatalf("civilDateTime(%q): expected error", tc.input)
		}
		if !strings.Contains(err.Error(), tc.wantErr) {
			t.Fatalf("civilDateTime(%q) error = %q, want substring %q", tc.input, err, tc.wantErr)
		}
	}
}

func TestHelpAndVersionWorkWithCorruptConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GHEALTH_CONFIG_DIR", dir)
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	if code := RunWithWriters([]string{"help"}, "test", &stdout, &stderr); code != ExitSuccess {
		t.Fatalf("help exit = %d, stderr = %s", code, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("Usage:")) {
		t.Fatalf("help output = %s", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := RunWithWriters([]string{"--version"}, "1.2.3-test", &stdout, &stderr); code != ExitSuccess {
		t.Fatalf("version exit = %d, stderr = %s", code, stderr.String())
	}
	if strings.TrimSpace(stdout.String()) != "1.2.3-test" {
		t.Fatalf("version output = %q", stdout.String())
	}

	// Other commands still surface the config error.
	stdout.Reset()
	stderr.Reset()
	if code := RunWithWriters([]string{"types", "list"}, "test", &stdout, &stderr); code == ExitSuccess {
		t.Fatalf("types list with corrupt config: expected failure, stdout = %s", stdout.String())
	}
}
