package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/julianknutsen/wasteland/internal/federation"
)

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Minute, "30m"},
		{59 * time.Minute, "59m"},
		{time.Hour, "1h"},
		{3 * time.Hour, "3h"},
		{23 * time.Hour, "23h"},
		{24 * time.Hour, "1d"},
		{72 * time.Hour, "3d"},
	}
	for _, tt := range tests {
		got := formatDuration(tt.d)
		if got != tt.want {
			t.Errorf("formatDuration(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

func TestWarnIfStale_NilLastSync(t *testing.T) {
	var buf bytes.Buffer
	cfg := &federation.Config{}
	warnIfStale(&buf, cfg)
	if buf.Len() != 0 {
		t.Errorf("expected no output for nil LastSyncAt, got: %s", buf.String())
	}
}

func TestWarnIfStale_Recent(t *testing.T) {
	var buf bytes.Buffer
	now := time.Now()
	cfg := &federation.Config{LastSyncAt: &now}
	warnIfStale(&buf, cfg)
	if buf.Len() != 0 {
		t.Errorf("expected no output for recent sync, got: %s", buf.String())
	}
}

func TestWarnIfStale_OldSync(t *testing.T) {
	var buf bytes.Buffer
	old := time.Now().Add(-3 * time.Hour)
	cfg := &federation.Config{LastSyncAt: &old}
	warnIfStale(&buf, cfg)
	if !strings.Contains(buf.String(), "Last synced") {
		t.Errorf("expected stale warning, got: %s", buf.String())
	}
	if !strings.Contains(buf.String(), "3h ago") {
		t.Errorf("expected '3h ago' in warning, got: %s", buf.String())
	}
}

func TestWarnIfStale_DaysOld(t *testing.T) {
	var buf bytes.Buffer
	old := time.Now().Add(-48 * time.Hour)
	cfg := &federation.Config{LastSyncAt: &old}
	warnIfStale(&buf, cfg)
	if !strings.Contains(buf.String(), "2d ago") {
		t.Errorf("expected '2d ago' in warning, got: %s", buf.String())
	}
}
