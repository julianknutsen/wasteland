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

type createRefCall struct {
	Repo, Ref, SHA string
}

type updateRefCall struct {
	Repo, Ref, SHA string
	Force          bool
}

type createPRCall struct {
	Repo, Title, Body, Head, Base string
}

type updatePRCall struct {
	Repo, Number string
	Fields       map[string]string
}

// fakeGitHubPRClient is a hand-written fake for unit testing GitHub PR operations.
type fakeGitHubPRClient struct {
	prs     map[string]fakePR // head → {url, number}
	reviews map[string][]byte // number → review JSON

	// Error injection — review & PR lifecycle
	SubmitReviewErr error
	ListReviewsErr  error
	ClosePRErr      error
	AddCommentErr   error
	DeleteRefErr    error

	// Error injection — git tree operations
	GetRefErr        error
	GetCommitTreeErr error
	CreateBlobErr    error
	CreateTreeErr    error
	CreateCommitErr  error
	CreateRefErr     error
	UpdateRefErr     error

	// Error injection — PR CRUD
	CreatePRErr error
	UpdatePRErr error

	// Return values — git tree operations
	GetRefSHA        string
	GetCommitTreeSHA string
	CreateBlobSHA    string
	CreateTreeSHA    string
	CreateCommitSHA  string
	CreatePRURL      string

	// Call tracking — review & PR lifecycle
	SubmitReviewCalls []submitReviewCall
	ClosePRCalls      []string // numbers
	AddCommentCalls   []addCommentCall
	DeleteRefCalls    []deleteRefCall

	// Call tracking — git tree + PR creation
	CreateRefCalls []createRefCall
	UpdateRefCalls []updateRefCall
	CreatePRCalls  []createPRCall
	UpdatePRCalls  []updatePRCall
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

func (f *fakeGitHubPRClient) GetRef(_, _ string) (string, error) {
	return f.GetRefSHA, f.GetRefErr
}

func (f *fakeGitHubPRClient) GetCommitTree(_, _ string) (string, error) {
	return f.GetCommitTreeSHA, f.GetCommitTreeErr
}

func (f *fakeGitHubPRClient) CreateBlob(_, _, _ string) (string, error) {
	return f.CreateBlobSHA, f.CreateBlobErr
}

func (f *fakeGitHubPRClient) CreateTree(_, _ string, _ []TreeEntry) (string, error) {
	return f.CreateTreeSHA, f.CreateTreeErr
}

func (f *fakeGitHubPRClient) CreateCommit(_, _, _ string, _ []string) (string, error) {
	return f.CreateCommitSHA, f.CreateCommitErr
}

func (f *fakeGitHubPRClient) CreateRef(repo, ref, sha string) error {
	f.CreateRefCalls = append(f.CreateRefCalls, createRefCall{repo, ref, sha})
	return f.CreateRefErr
}

func (f *fakeGitHubPRClient) UpdateRef(repo, ref, sha string, force bool) error {
	f.UpdateRefCalls = append(f.UpdateRefCalls, updateRefCall{repo, ref, sha, force})
	return f.UpdateRefErr
}

func (f *fakeGitHubPRClient) CreatePR(repo, title, body, head, base string) (string, error) {
	f.CreatePRCalls = append(f.CreatePRCalls, createPRCall{repo, title, body, head, base})
	return f.CreatePRURL, f.CreatePRErr
}

func (f *fakeGitHubPRClient) UpdatePR(repo, number string, fields map[string]string) error {
	f.UpdatePRCalls = append(f.UpdatePRCalls, updatePRCall{repo, number, fields})
	return f.UpdatePRErr
}
