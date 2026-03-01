package inference

import "fmt"

// VerifyResult holds the outcome of re-running an inference job and comparing hashes.
type VerifyResult struct {
	Match        bool
	ExpectedHash string
	ActualHash   string
	Output       string
}

// Verify re-runs the inference job and compares the output hash against the claimed result.
func Verify(j *Job, r *Result) (*VerifyResult, error) {
	actual, err := Run(j)
	if err != nil {
		return nil, fmt.Errorf("verification re-run failed: %w", err)
	}

	return &VerifyResult{
		Match:        actual.OutputHash == r.OutputHash,
		ExpectedHash: r.OutputHash,
		ActualHash:   actual.OutputHash,
		Output:       actual.Output,
	}, nil
}
