package main

import (
	"encoding/json"
	"fmt"
)

// GitHubPRClient abstracts GitHub PR operations for testability.
type GitHubPRClient interface {
	FindPR(repo, head string) (url, number string)
	SubmitReview(repo, number, event, body string) error
	ListReviews(repo, number string) ([]byte, error)
	ClosePR(repo, number string) error
	AddComment(repo, number, body string) error
	DeleteRef(repo, ref string) error
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
