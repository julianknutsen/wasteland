package inference

import (
	"encoding/json"
	"fmt"
)

// EncodeJob serializes a Job to JSON for the wanted item description field.
func EncodeJob(j *Job) (string, error) {
	if j.Prompt == "" {
		return "", fmt.Errorf("inference job: prompt is required")
	}
	if j.Model == "" {
		return "", fmt.Errorf("inference job: model is required")
	}
	b, err := json.Marshal(j)
	if err != nil {
		return "", fmt.Errorf("encoding inference job: %w", err)
	}
	return string(b), nil
}

// DecodeJob parses a Job from the wanted item description field.
func DecodeJob(s string) (*Job, error) {
	var j Job
	if err := json.Unmarshal([]byte(s), &j); err != nil {
		return nil, fmt.Errorf("decoding inference job: %w", err)
	}
	if j.Prompt == "" {
		return nil, fmt.Errorf("inference job: prompt is required")
	}
	if j.Model == "" {
		return nil, fmt.Errorf("inference job: model is required")
	}
	return &j, nil
}

// EncodeResult serializes a Result to JSON for the completion evidence field.
func EncodeResult(r *Result) (string, error) {
	if r.Output == "" {
		return "", fmt.Errorf("inference result: output is required")
	}
	if r.OutputHash == "" {
		return "", fmt.Errorf("inference result: output_hash is required")
	}
	if r.Model == "" {
		return "", fmt.Errorf("inference result: model is required")
	}
	b, err := json.Marshal(r)
	if err != nil {
		return "", fmt.Errorf("encoding inference result: %w", err)
	}
	return string(b), nil
}

// DecodeResult parses a Result from the completion evidence field.
func DecodeResult(s string) (*Result, error) {
	var r Result
	if err := json.Unmarshal([]byte(s), &r); err != nil {
		return nil, fmt.Errorf("decoding inference result: %w", err)
	}
	if r.Output == "" {
		return nil, fmt.Errorf("inference result: output is required")
	}
	if r.OutputHash == "" {
		return nil, fmt.Errorf("inference result: output_hash is required")
	}
	if r.Model == "" {
		return nil, fmt.Errorf("inference result: model is required")
	}
	return &r, nil
}
