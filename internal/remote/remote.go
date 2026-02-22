// Package remote abstracts how dolt databases are addressed and forked.
//
// Implementations include DoltHub (production), file:// (testing/offline),
// and git bare repos (future).
package remote

// Provider abstracts how dolt databases are addressed and forked.
type Provider interface {
	// DatabaseURL returns the dolt-compatible remote URL for org/db.
	DatabaseURL(org, db string) string

	// Fork creates a copy of a database from one org to another.
	Fork(fromOrg, fromDB, toOrg string) error

	// Type returns a label for logging ("dolthub", "file", "git").
	Type() string
}
