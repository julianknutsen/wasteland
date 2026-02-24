// Package schema provides the canonical commons database schema.
package schema

import _ "embed"

// SQL contains the commons schema DDL (CREATE TABLEs and seed data).
// It does not include DOLT_ADD/DOLT_COMMIT — callers handle that.
//
//go:embed commons.sql
var SQL string
