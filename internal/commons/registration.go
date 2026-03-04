package commons

import "fmt"

// BuildRegistrationSQL generates the SQL statement to register a rig.
// Used by both the CLI join command and the hosted connect flow.
func BuildRegistrationSQL(handle, dolthubOrg, displayName, ownerEmail, version string) string {
	hopURI := fmt.Sprintf("hop://%s/%s/", ownerEmail, handle)
	return fmt.Sprintf(
		`INSERT INTO rigs (handle, display_name, dolthub_org, hop_uri, owner_email, gt_version, trust_level, registered_at, last_seen) `+
			`VALUES ('%s', '%s', '%s', '%s', '%s', '%s', 1, NOW(), NOW()) `+
			`ON DUPLICATE KEY UPDATE display_name = '%s', dolthub_org = '%s', hop_uri = '%s', owner_email = '%s', gt_version = '%s', last_seen = NOW()`,
		EscapeSQL(handle),
		EscapeSQL(displayName),
		EscapeSQL(dolthubOrg),
		EscapeSQL(hopURI),
		EscapeSQL(ownerEmail),
		EscapeSQL(version),
		EscapeSQL(displayName),
		EscapeSQL(dolthubOrg),
		EscapeSQL(hopURI),
		EscapeSQL(ownerEmail),
		EscapeSQL(version),
	)
}
