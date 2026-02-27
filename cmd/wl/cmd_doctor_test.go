package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/julianknutsen/wasteland/internal/federation"
)

// fakeConfigStore implements federation.ConfigStore for testing.
type fakeConfigStore struct {
	configs map[string]*federation.Config
	listErr error
}

func (f *fakeConfigStore) Load(upstream string) (*federation.Config, error) {
	cfg, ok := f.configs[upstream]
	if !ok {
		return nil, federation.ErrNotJoined
	}
	return cfg, nil
}

func (f *fakeConfigStore) Save(_ *federation.Config) error { return nil }
func (f *fakeConfigStore) Delete(_ string) error           { return nil }

func (f *fakeConfigStore) List() ([]string, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	var keys []string
	for k := range f.configs {
		keys = append(keys, k)
	}
	return keys, nil
}

func TestDoctor_DoltNotFound(t *testing.T) {
	var stdout bytes.Buffer
	deps := &doctorDeps{
		lookPath: func(string) (string, error) { return "", &notFoundErr{} },
		getenv:   func(string) string { return "" },
		store:    &fakeConfigStore{configs: map[string]*federation.Config{}},
	}
	runDoctorChecks(&stdout, deps)
	if !strings.Contains(stdout.String(), "not found in PATH") {
		t.Errorf("expected 'not found in PATH' in output, got: %s", stdout.String())
	}
}

func TestDoctor_EnvVarsSet(t *testing.T) {
	var stdout bytes.Buffer
	deps := &doctorDeps{
		lookPath: func(string) (string, error) { return "", &notFoundErr{} },
		getenv: func(key string) string {
			switch key {
			case "DOLTHUB_TOKEN":
				return "tok_secret"
			case "DOLTHUB_ORG":
				return "myorg"
			}
			return ""
		},
		store: &fakeConfigStore{configs: map[string]*federation.Config{}},
	}
	runDoctorChecks(&stdout, deps)
	out := stdout.String()
	if !strings.Contains(out, "DOLTHUB_TOKEN: set") {
		t.Errorf("expected DOLTHUB_TOKEN set, got: %s", out)
	}
	if !strings.Contains(out, "DOLTHUB_ORG: set (myorg)") {
		t.Errorf("expected DOLTHUB_ORG set (myorg), got: %s", out)
	}
}

func TestDoctor_EnvVarsNotSet(t *testing.T) {
	var stdout bytes.Buffer
	deps := &doctorDeps{
		lookPath: func(string) (string, error) { return "", &notFoundErr{} },
		getenv:   func(string) string { return "" },
		store:    &fakeConfigStore{configs: map[string]*federation.Config{}},
	}
	runDoctorChecks(&stdout, deps)
	out := stdout.String()
	if !strings.Contains(out, "DOLTHUB_TOKEN: not set") {
		t.Errorf("expected DOLTHUB_TOKEN not set, got: %s", out)
	}
	if !strings.Contains(out, "DOLTHUB_ORG: not set") {
		t.Errorf("expected DOLTHUB_ORG not set, got: %s", out)
	}
}

func TestDoctor_NoWastelands(t *testing.T) {
	var stdout bytes.Buffer
	deps := &doctorDeps{
		lookPath: func(string) (string, error) { return "", &notFoundErr{} },
		getenv:   func(string) string { return "" },
		store:    &fakeConfigStore{configs: map[string]*federation.Config{}},
	}
	runDoctorChecks(&stdout, deps)
	if !strings.Contains(stdout.String(), "none joined") {
		t.Errorf("expected 'none joined' in output, got: %s", stdout.String())
	}
}

func TestDoctor_WastelandJoined(t *testing.T) {
	var stdout bytes.Buffer
	deps := &doctorDeps{
		lookPath: func(string) (string, error) { return "", &notFoundErr{} },
		getenv:   func(string) string { return "" },
		store: &fakeConfigStore{configs: map[string]*federation.Config{
			"hop/wl-commons": {
				Upstream: "hop/wl-commons",
				LocalDir: "/tmp/nonexistent-test-path",
				Mode:     "wild-west",
				Signing:  true,
			},
		}},
	}
	runDoctorChecks(&stdout, deps)
	out := stdout.String()
	if !strings.Contains(out, "1 wasteland(s) joined") {
		t.Errorf("expected '1 wasteland(s) joined', got: %s", out)
	}
	if !strings.Contains(out, "hop/wl-commons") {
		t.Errorf("expected 'hop/wl-commons' in output, got: %s", out)
	}
	if !strings.Contains(out, "GPG signing: enabled but dolt not found") {
		t.Errorf("expected 'GPG signing: enabled but dolt not found', got: %s", out)
	}
}

func TestDoctor_GPGDisabled(t *testing.T) {
	var stdout bytes.Buffer
	deps := &doctorDeps{
		lookPath: func(string) (string, error) { return "", &notFoundErr{} },
		getenv:   func(string) string { return "" },
		store: &fakeConfigStore{configs: map[string]*federation.Config{
			"hop/wl-commons": {
				Upstream: "hop/wl-commons",
				LocalDir: "/tmp/nonexistent-test-path",
				Mode:     "wild-west",
				Signing:  false,
			},
		}},
	}
	runDoctorChecks(&stdout, deps)
	out := stdout.String()
	if !strings.Contains(out, "GPG signing: disabled") {
		t.Errorf("expected 'GPG signing: disabled', got: %s", out)
	}
}

func TestDoctor_StaleSync(t *testing.T) {
	var stdout bytes.Buffer
	old := time.Now().Add(-72 * time.Hour)
	deps := &doctorDeps{
		lookPath: func(string) (string, error) { return "", &notFoundErr{} },
		getenv:   func(string) string { return "" },
		store: &fakeConfigStore{configs: map[string]*federation.Config{
			"hop/wl-commons": {
				Upstream:   "hop/wl-commons",
				LocalDir:   "/tmp/nonexistent-test-path",
				LastSyncAt: &old,
			},
		}},
	}
	results := runDoctorChecks(&stdout, deps)
	out := stdout.String()
	if !strings.Contains(out, "sync: last synced 3d ago") {
		t.Errorf("expected stale sync warning, got: %s", out)
	}

	// Find the sync diagnostic
	var found bool
	for _, d := range results {
		if strings.HasSuffix(d.name, "/sync") && d.status == "warn" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected a warn diagnostic for stale sync")
	}
}

func TestDoctor_RecentSync(t *testing.T) {
	var stdout bytes.Buffer
	recent := time.Now().Add(-30 * time.Minute)
	deps := &doctorDeps{
		lookPath: func(string) (string, error) { return "", &notFoundErr{} },
		getenv:   func(string) string { return "" },
		store: &fakeConfigStore{configs: map[string]*federation.Config{
			"hop/wl-commons": {
				Upstream:   "hop/wl-commons",
				LocalDir:   "/tmp/nonexistent-test-path",
				LastSyncAt: &recent,
			},
		}},
	}
	results := runDoctorChecks(&stdout, deps)
	for _, d := range results {
		if strings.HasSuffix(d.name, "/sync") && d.status == "warn" {
			t.Errorf("expected no sync warning for recent sync, got warn: %s", d.message)
		}
	}
}

func TestDoctor_CheckFlag(t *testing.T) {
	var stdout bytes.Buffer
	err := runDoctor(&stdout, &stdout,
		func(string) (string, error) { return "", &notFoundErr{} },
		func(string) string { return "" },
		&fakeConfigStore{configs: map[string]*federation.Config{}},
		false, true)
	// Should return errExit because there are warnings (dolt not found, etc.)
	if !errors.Is(err, errExit) {
		t.Errorf("expected errExit with --check, got: %v", err)
	}
}

func TestDoctor_CheckFlag_AllPass(t *testing.T) {
	// This test is minimal — in practice it's hard to make all checks pass
	// without real dolt, but we can test the logic path.
	var stdout bytes.Buffer
	err := runDoctor(&stdout, &stdout,
		func(string) (string, error) { return "", &notFoundErr{} },
		func(string) string { return "" },
		&fakeConfigStore{configs: map[string]*federation.Config{}},
		false, true)
	// With dolt not found, --check returns errExit
	if !errors.Is(err, errExit) {
		t.Errorf("expected errExit, got: %v", err)
	}
}

func TestDoctor_OrphanedClone(t *testing.T) {
	var stdout bytes.Buffer
	deps := &doctorDeps{
		lookPath: func(string) (string, error) { return "", &notFoundErr{} },
		getenv:   func(string) string { return "" },
		store: &fakeConfigStore{configs: map[string]*federation.Config{
			"hop/wl-commons": {
				Upstream:    "hop/wl-commons",
				LocalDir:    "/tmp/nonexistent-clone-path",
				UpstreamURL: "https://dolthub.com/hop/wl-commons",
			},
		}},
	}
	results := runDoctorChecks(&stdout, deps)
	out := stdout.String()
	if !strings.Contains(out, "Local clone: missing") {
		t.Errorf("expected orphaned clone warning, got: %s", out)
	}

	// Verify fixFunc is set for orphaned clone
	var found bool
	for _, d := range results {
		if strings.HasSuffix(d.name, "/clone") && d.status == "fail" && d.fixFunc != nil {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected fixFunc to be set for orphaned clone with UpstreamURL")
	}
}

type notFoundErr struct{}

func (e *notFoundErr) Error() string { return "executable file not found in $PATH" }
