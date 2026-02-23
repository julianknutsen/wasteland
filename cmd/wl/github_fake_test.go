package main

type fakePR struct {
	URL    string
	Number string
}

type submitReviewCall struct {
	Repo, Number, Event, Body string
}

type addCommentCall struct {
	Repo, Number, Body string
}

type deleteRefCall struct {
	Repo, Ref string
}

// fakeGitHubPRClient is a hand-written fake for unit testing GitHub PR operations.
type fakeGitHubPRClient struct {
	prs     map[string]fakePR // head → {url, number}
	reviews map[string][]byte // number → review JSON

	// Error injection (one per method)
	SubmitReviewErr error
	ListReviewsErr  error
	ClosePRErr      error
	AddCommentErr   error
	DeleteRefErr    error

	// Call tracking
	SubmitReviewCalls []submitReviewCall
	ClosePRCalls      []string // numbers
	AddCommentCalls   []addCommentCall
	DeleteRefCalls    []deleteRefCall
}

// compile-time check
var _ GitHubPRClient = (*fakeGitHubPRClient)(nil)

func (f *fakeGitHubPRClient) FindPR(_, head string) (url, number string) {
	pr, ok := f.prs[head]
	if !ok {
		return "", ""
	}
	return pr.URL, pr.Number
}

func (f *fakeGitHubPRClient) SubmitReview(repo, number, event, body string) error {
	f.SubmitReviewCalls = append(f.SubmitReviewCalls, submitReviewCall{repo, number, event, body})
	return f.SubmitReviewErr
}

func (f *fakeGitHubPRClient) ListReviews(_, number string) ([]byte, error) {
	if f.ListReviewsErr != nil {
		return nil, f.ListReviewsErr
	}
	data, ok := f.reviews[number]
	if !ok {
		return []byte("[]"), nil
	}
	return data, nil
}

func (f *fakeGitHubPRClient) ClosePR(_, number string) error {
	f.ClosePRCalls = append(f.ClosePRCalls, number)
	return f.ClosePRErr
}

func (f *fakeGitHubPRClient) AddComment(repo, number, body string) error {
	f.AddCommentCalls = append(f.AddCommentCalls, addCommentCall{repo, number, body})
	return f.AddCommentErr
}

func (f *fakeGitHubPRClient) DeleteRef(repo, ref string) error {
	f.DeleteRefCalls = append(f.DeleteRefCalls, deleteRefCall{repo, ref})
	return f.DeleteRefErr
}
