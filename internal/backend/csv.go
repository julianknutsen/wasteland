package backend

import (
	"encoding/json"
	"fmt"
	"strings"
)

// queryResponse is the JSON shape returned by the DoltHub SQL API.
type queryResponse struct {
	QueryExecutionStatus  string            `json:"query_execution_status"`
	QueryExecutionMessage string            `json:"query_execution_message"`
	RepositoryOwner       string            `json:"repository_owner"`
	RepositoryName        string            `json:"repository_name"`
	CommitRef             string            `json:"commit_ref"`
	SQLQuery              string            `json:"sql_query"`
	SchemaFragment        json.RawMessage   `json:"schema_fragment"`
	Rows                  []json.RawMessage `json:"rows"`
}

// JSONToCSV converts a DoltHub API JSON response to CSV format matching
// dolt sql -r csv output. Column order comes from schema_fragment.
func JSONToCSV(jsonResp []byte) (string, error) {
	var resp queryResponse
	if err := json.Unmarshal(jsonResp, &resp); err != nil {
		return "", fmt.Errorf("parsing API response: %w", err)
	}

	if resp.QueryExecutionStatus == "Error" {
		return "", fmt.Errorf("query error: %s", resp.QueryExecutionMessage)
	}

	// Extract column order from schema_fragment.
	columns, err := extractColumns(resp.SchemaFragment)
	if err != nil {
		// Fall back to extracting column order from first row.
		if len(resp.Rows) > 0 {
			columns, err = extractColumnsFromRow(resp.Rows[0])
			if err != nil {
				return "", fmt.Errorf("cannot determine column order: %w", err)
			}
		} else {
			return "", nil // no schema, no rows = empty result
		}
	}

	if len(columns) == 0 {
		return "", nil
	}

	var b strings.Builder

	// Header line.
	b.WriteString(strings.Join(columns, ","))
	b.WriteByte('\n')

	// Data rows.
	for _, rawRow := range resp.Rows {
		var row map[string]any
		if err := json.Unmarshal(rawRow, &row); err != nil {
			continue
		}
		for i, col := range columns {
			if i > 0 {
				b.WriteByte(',')
			}
			val, ok := row[col]
			if !ok || val == nil {
				// Empty string for NULL (matches dolt CSV output).
				continue
			}
			b.WriteString(formatCSVField(val))
		}
		b.WriteByte('\n')
	}

	return b.String(), nil
}

// extractColumns parses the schema_fragment to get ordered column names.
// DoltHub schema_fragment format: [{"columnName":"id","columnType":"varchar(...)"},...]
func extractColumns(schema json.RawMessage) ([]string, error) {
	if len(schema) == 0 {
		return nil, fmt.Errorf("empty schema")
	}

	var cols []struct {
		ColumnName string `json:"columnName"`
	}
	if err := json.Unmarshal(schema, &cols); err != nil {
		return nil, err
	}
	if len(cols) == 0 {
		return nil, fmt.Errorf("no columns in schema")
	}

	names := make([]string, len(cols))
	for i, c := range cols {
		names[i] = c.ColumnName
	}
	return names, nil
}

// extractColumnsFromRow extracts column names from a JSON row object.
// Order is not guaranteed by JSON spec, but Go's json.Decoder preserves
// insertion order for most practical cases. As a fallback this is acceptable.
func extractColumnsFromRow(raw json.RawMessage) ([]string, error) {
	var row map[string]any
	if err := json.Unmarshal(raw, &row); err != nil {
		return nil, err
	}
	// Use json.Decoder to get key order.
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	t, err := dec.Token() // opening {
	if err != nil {
		return nil, err
	}
	if t != json.Delim('{') {
		return nil, fmt.Errorf("expected object")
	}
	var keys []string
	for dec.More() {
		t, err := dec.Token()
		if err != nil {
			break
		}
		key, ok := t.(string)
		if !ok {
			break
		}
		keys = append(keys, key)
		// Skip value.
		var v json.RawMessage
		if err := dec.Decode(&v); err != nil {
			break
		}
	}
	return keys, nil
}

// formatCSVField formats a value for CSV output, quoting if needed.
func formatCSVField(val any) string {
	var s string
	switch v := val.(type) {
	case string:
		s = v
	case float64:
		// JSON numbers are float64; format integers without decimal.
		if v == float64(int64(v)) {
			s = fmt.Sprintf("%d", int64(v))
		} else {
			s = fmt.Sprintf("%g", v)
		}
	case bool:
		if v {
			s = "1"
		} else {
			s = "0"
		}
	default:
		// For objects/arrays (like JSON fields), marshal back to string.
		b, err := json.Marshal(v)
		if err != nil {
			s = fmt.Sprintf("%v", v)
		} else {
			s = string(b)
		}
	}

	// Quote if contains comma, newline, or double-quote.
	if strings.ContainsAny(s, ",\n\"") {
		s = "\"" + strings.ReplaceAll(s, "\"", "\"\"") + "\""
	}
	return s
}
