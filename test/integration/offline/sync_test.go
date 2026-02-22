//go:build integration

package offline

import (
	"strings"
	"testing"
)

func TestSyncFromUpstream(t *testing.T) {
	for _, backend := range backends {
		t.Run(string(backend), func(t *testing.T) {
			env := newTestEnv(t, backend)
			env.createUpstreamStoreWithData(t, upstreamOrg, upstreamDB)
			env.joinWasteland(t, upstream, forkOrg)

			// Verify seed data is in the fork clone (local_dir from config).
			cfg := env.loadConfig(t, upstream)
			forkDir := cfg["local_dir"].(string)

			raw := doltSQL(t, forkDir, "SELECT COUNT(*) FROM wanted WHERE id='w-seed001'")
			rows := parseCSV(t, raw)
			if len(rows) < 2 || rows[1][0] != "1" {
				t.Fatal("seed data not found in fork after join")
			}

			// Push new data to the upstream remote store.
			newDataSQL := `INSERT INTO wanted (id, title, status, type, priority, effort_level, created_at, updated_at)
VALUES ('w-new0001', 'New upstream item', 'open', 'bug', 1, 'small', NOW(), NOW());
CALL DOLT_ADD('-A');
CALL DOLT_COMMIT('-m', 'Add new upstream data');
`
			env.pushToUpstreamStore(t, upstreamOrg, upstreamDB, newDataSQL)

			// Verify fork does NOT have the new data yet.
			raw = doltSQL(t, forkDir, "SELECT COUNT(*) FROM wanted WHERE id='w-new0001'")
			rows = parseCSV(t, raw)
			if len(rows) >= 2 && rows[1][0] != "0" {
				t.Fatal("fork should not have new data before sync")
			}

			// Run wl sync.
			stdout, stderr, err := runWL(t, env, "sync")
			if err != nil {
				t.Fatalf("wl sync failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
			}

			// Verify fork now has the new data.
			raw = doltSQL(t, forkDir, "SELECT COUNT(*) FROM wanted WHERE id='w-new0001'")
			rows = parseCSV(t, raw)
			if len(rows) < 2 || rows[1][0] != "1" {
				t.Errorf("fork should have new data after sync; got count=%s", rows[1][0])
			}
		})
	}
}

func TestSyncDryRun(t *testing.T) {
	for _, backend := range backends {
		t.Run(string(backend), func(t *testing.T) {
			env := newTestEnv(t, backend)
			env.createUpstreamStoreWithData(t, upstreamOrg, upstreamDB)
			env.joinWasteland(t, upstream, forkOrg)

			cfg := env.loadConfig(t, upstream)
			forkDir := cfg["local_dir"].(string)

			// Push new data to upstream.
			newDataSQL := `INSERT INTO wanted (id, title, status, type, priority, effort_level, created_at, updated_at)
VALUES ('w-dry0001', 'Dry run item', 'open', 'feature', 2, 'medium', NOW(), NOW());
CALL DOLT_ADD('-A');
CALL DOLT_COMMIT('-m', 'Add dry run data');
`
			env.pushToUpstreamStore(t, upstreamOrg, upstreamDB, newDataSQL)

			// Run wl sync --dry-run.
			stdout, stderr, err := runWL(t, env, "sync", "--dry-run")
			if err != nil {
				t.Fatalf("wl sync --dry-run failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
			}

			// Output should mention dry run.
			if !strings.Contains(stdout, "Dry run") {
				t.Errorf("output should mention dry run: %s", stdout)
			}

			// Fork should NOT have the new data.
			raw := doltSQL(t, forkDir, "SELECT COUNT(*) FROM wanted WHERE id='w-dry0001'")
			rows := parseCSV(t, raw)
			if len(rows) >= 2 && rows[1][0] != "0" {
				t.Errorf("fork should NOT have new data after dry-run sync; got count=%s", rows[1][0])
			}
		})
	}
}
