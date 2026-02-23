package main

import (
	"encoding/json"
	"fmt"
)

// TreeEntry describes a single file entry for CreateTree.
type TreeEntry struct {
	Path string
	Mode string
	Type string
	SHA  string
}

// GitHubPRClient abstracts GitHub PR operations for testability.
type GitHubPRClient interface {
	// Review & PR lifecycle (phase 5)
	FindPR(repo, head string) (url, number string)
	SubmitReview(repo, number, event, body string) error
	ListReviews(repo, number string) ([]byte, error)
	ClosePR(repo, number string) error
	AddComment(repo, number, body string) error
	DeleteRef(repo, ref string) error

	// Git tree operations (PR creation)
	GetRef(repo, ref string) (sha string, err error)
	GetCommitTree(repo, commitSHA string) (treeSHA string, err error)
	CreateBlob(repo, content, encoding string) (sha string, err error)
	CreateTree(repo, baseTreeSHA string, entries []TreeEntry) (sha string, err error)
	CreateCommit(repo, message, treeSHA string, parents []string) (sha string, err error)
	CreateRef(repo, ref, sha string) error
	UpdateRef(repo, ref, sha string, force bool) error

	// PR CRUD
	CreatePR(repo, title, body, head, base string) (htmlURL string, err error)
	UpdatePR(repo, number string, fields map[string]string) error
}

// ghCLIClient implements GitHubPRClient using the gh CLI.
type ghCLIClient struct {
	ghPath string
}

func newGHClient(ghPath string) *ghCLIClient {
	return &ghCLIClient{ghPath: ghPath}
}

func (c *ghCLIClient) FindPR(repo, head string) (url, number string) {
	return findExistingPR(c.ghPath, repo, head)
}

func (c *ghCLIClient) SubmitReview(repo, number, event, body string) error {
	reviewBody, _ := json.Marshal(map[string]string{
		"event": event,
		"body":  body,
	})
	_, err := ghAPICall(c.ghPath, "POST", fmt.Sprintf("repos/%s/pulls/%s/reviews", repo, number), string(reviewBody))
	return err
}

func (c *ghCLIClient) ListReviews(repo, number string) ([]byte, error) {
	return ghAPICall(c.ghPath, "GET", fmt.Sprintf("repos/%s/pulls/%s/reviews", repo, number), "")
}

func (c *ghCLIClient) ClosePR(repo, number string) error {
	closeBody, _ := json.Marshal(map[string]string{
		"state": "closed",
	})
	_, err := ghAPICall(c.ghPath, "PATCH", fmt.Sprintf("repos/%s/pulls/%s", repo, number), string(closeBody))
	return err
}

func (c *ghCLIClient) AddComment(repo, number, body string) error {
	commentBody, _ := json.Marshal(map[string]string{
		"body": body,
	})
	_, err := ghAPICall(c.ghPath, "POST", fmt.Sprintf("repos/%s/issues/%s/comments", repo, number), string(commentBody))
	return err
}

func (c *ghCLIClient) DeleteRef(repo, ref string) error {
	_, err := ghAPICall(c.ghPath, "DELETE", fmt.Sprintf("repos/%s/git/refs/%s", repo, ref), "")
	return err
}

func (c *ghCLIClient) GetRef(repo, ref string) (string, error) {
	data, err := ghAPICall(c.ghPath, "GET", fmt.Sprintf("repos/%s/git/ref/%s", repo, ref), "")
	if err != nil {
		return "", err
	}
	var result struct {
		Object struct {
			SHA string `json:"sha"`
		} `json:"object"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", fmt.Errorf("parsing ref response: %w", err)
	}
	return result.Object.SHA, nil
}

func (c *ghCLIClient) GetCommitTree(repo, commitSHA string) (string, error) {
	data, err := ghAPICall(c.ghPath, "GET", fmt.Sprintf("repos/%s/git/commits/%s", repo, commitSHA), "")
	if err != nil {
		return "", err
	}
	var result struct {
		Tree struct {
			SHA string `json:"sha"`
		} `json:"tree"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", fmt.Errorf("parsing commit response: %w", err)
	}
	return result.Tree.SHA, nil
}

func (c *ghCLIClient) CreateBlob(repo, content, encoding string) (string, error) {
	body, _ := json.Marshal(map[string]string{
		"content":  content,
		"encoding": encoding,
	})
	data, err := ghAPICall(c.ghPath, "POST", fmt.Sprintf("repos/%s/git/blobs", repo), string(body))
	if err != nil {
		return "", err
	}
	var result struct {
		SHA string `json:"sha"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", fmt.Errorf("parsing blob response: %w", err)
	}
	return result.SHA, nil
}

func (c *ghCLIClient) CreateTree(repo, baseTreeSHA string, entries []TreeEntry) (string, error) {
	apiEntries := make([]map[string]string, len(entries))
	for i, e := range entries {
		apiEntries[i] = map[string]string{
			"path": e.Path,
			"mode": e.Mode,
			"type": e.Type,
			"sha":  e.SHA,
		}
	}
	body, _ := json.Marshal(map[string]interface{}{
		"base_tree": baseTreeSHA,
		"tree":      apiEntries,
	})
	data, err := ghAPICall(c.ghPath, "POST", fmt.Sprintf("repos/%s/git/trees", repo), string(body))
	if err != nil {
		return "", err
	}
	var result struct {
		SHA string `json:"sha"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", fmt.Errorf("parsing tree response: %w", err)
	}
	return result.SHA, nil
}

func (c *ghCLIClient) CreateCommit(repo, message, treeSHA string, parents []string) (string, error) {
	body, _ := json.Marshal(map[string]interface{}{
		"message": message,
		"tree":    treeSHA,
		"parents": parents,
	})
	data, err := ghAPICall(c.ghPath, "POST", fmt.Sprintf("repos/%s/git/commits", repo), string(body))
	if err != nil {
		return "", err
	}
	var result struct {
		SHA string `json:"sha"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", fmt.Errorf("parsing commit response: %w", err)
	}
	return result.SHA, nil
}

func (c *ghCLIClient) CreateRef(repo, ref, sha string) error {
	body, _ := json.Marshal(map[string]string{
		"ref": ref,
		"sha": sha,
	})
	_, err := ghAPICall(c.ghPath, "POST", fmt.Sprintf("repos/%s/git/refs", repo), string(body))
	return err
}

func (c *ghCLIClient) UpdateRef(repo, ref, sha string, force bool) error {
	body, _ := json.Marshal(map[string]interface{}{
		"sha":   sha,
		"force": force,
	})
	_, err := ghAPICall(c.ghPath, "PATCH", fmt.Sprintf("repos/%s/git/refs/%s", repo, ref), string(body))
	return err
}

func (c *ghCLIClient) CreatePR(repo, title, body, head, base string) (string, error) {
	prBody, _ := json.Marshal(map[string]string{
		"title": title,
		"body":  body,
		"head":  head,
		"base":  base,
	})
	data, err := ghAPICall(c.ghPath, "POST", fmt.Sprintf("repos/%s/pulls", repo), string(prBody))
	if err != nil {
		return "", err
	}
	var result struct {
		HTMLURL string `json:"html_url"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", fmt.Errorf("parsing PR response: %w", err)
	}
	return result.HTMLURL, nil
}

func (c *ghCLIClient) UpdatePR(repo, number string, fields map[string]string) error {
	body, _ := json.Marshal(fields)
	_, err := ghAPICall(c.ghPath, "PATCH", fmt.Sprintf("repos/%s/pulls/%s", repo, number), string(body))
	return err
}
