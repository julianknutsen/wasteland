package federation

import (
	"fmt"
	"sort"
	"sync"
)

// CallLog is a shared ordered log for recording cross-component call sequences.
type CallLog struct {
	mu    sync.Mutex
	Calls []string
}

func NewCallLog() *CallLog {
	return &CallLog{}
}

func (l *CallLog) Record(call string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.Calls = append(l.Calls, call)
}

// FakeProvider is a test double for remote.Provider.
type FakeProvider struct {
	mu          sync.Mutex
	Forked      map[string]bool // "fromOrg/fromDB->toOrg" => true
	PRs         []string        // URLs of created PRs
	Calls       []string
	Log         *CallLog // shared ordered log (optional)
	ForkErr     error
	CreatePRErr error
	BaseURL     string // returned by DatabaseURL (default "https://fake-remote")
}

func NewFakeProvider() *FakeProvider {
	return &FakeProvider{
		Forked:  make(map[string]bool),
		BaseURL: "https://fake-remote",
	}
}

func (f *FakeProvider) DatabaseURL(org, db string) string {
	return fmt.Sprintf("%s/%s/%s", f.BaseURL, org, db)
}

func (f *FakeProvider) Fork(fromOrg, fromDB, toOrg string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	call := fmt.Sprintf("Fork(%s, %s, %s)", fromOrg, fromDB, toOrg)
	f.Calls = append(f.Calls, call)
	if f.Log != nil {
		f.Log.Record(call)
	}
	if f.ForkErr != nil {
		return f.ForkErr
	}
	f.Forked[fmt.Sprintf("%s/%s->%s", fromOrg, fromDB, toOrg)] = true
	return nil
}

func (f *FakeProvider) CreatePR(forkOrg, upstreamOrg, db, fromBranch, title, _ string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	call := fmt.Sprintf("CreatePR(%s, %s, %s, %s, %s)", forkOrg, upstreamOrg, db, fromBranch, title)
	f.Calls = append(f.Calls, call)
	if f.Log != nil {
		f.Log.Record(call)
	}
	if f.CreatePRErr != nil {
		return "", f.CreatePRErr
	}
	url := fmt.Sprintf("https://fake-pr/%s/%s/pulls/1", upstreamOrg, db)
	f.PRs = append(f.PRs, url)
	return url, nil
}

func (f *FakeProvider) Type() string { return "fake" }

// FakeDoltCLI is a test double for DoltCLI.
type FakeDoltCLI struct {
	mu         sync.Mutex
	Cloned     map[string]bool // "remoteURL -> targetDir"
	Registered map[string]bool // "handle"
	Pushed     map[string]bool // "localDir"
	Branches   map[string]bool // "localDir -> branch"
	Remotes    map[string]bool // "localDir -> remoteURL"
	Calls      []string
	Log        *CallLog // shared ordered log (optional)

	CloneErr      error
	CloneErrCount int // if > 0, CloneErr clears after this many calls
	cloneAttempts int
	RegisterErr   error
	PushErr       error
	RemoteErr     error
}

func NewFakeDoltCLI() *FakeDoltCLI {
	return &FakeDoltCLI{
		Cloned:     make(map[string]bool),
		Registered: make(map[string]bool),
		Pushed:     make(map[string]bool),
		Branches:   make(map[string]bool),
		Remotes:    make(map[string]bool),
	}
}

func (f *FakeDoltCLI) Clone(remoteURL, targetDir string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	call := fmt.Sprintf("Clone(%s, %s)", remoteURL, targetDir)
	f.Calls = append(f.Calls, call)
	if f.Log != nil {
		f.Log.Record(call)
	}
	if f.CloneErr != nil {
		f.cloneAttempts++
		if f.CloneErrCount > 0 && f.cloneAttempts >= f.CloneErrCount {
			f.CloneErr = nil
		} else {
			return f.CloneErr
		}
	}
	f.Cloned[fmt.Sprintf("%s->%s", remoteURL, targetDir)] = true
	return nil
}

func (f *FakeDoltCLI) RegisterRig(localDir, handle, _, _, _, _ string, _ bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	call := fmt.Sprintf("RegisterRig(%s, %s)", localDir, handle)
	f.Calls = append(f.Calls, call)
	if f.Log != nil {
		f.Log.Record(call)
	}
	if f.RegisterErr != nil {
		return f.RegisterErr
	}
	f.Registered[handle] = true
	return nil
}

func (f *FakeDoltCLI) Push(localDir string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	call := fmt.Sprintf("Push(%s)", localDir)
	f.Calls = append(f.Calls, call)
	if f.Log != nil {
		f.Log.Record(call)
	}
	if f.PushErr != nil {
		return f.PushErr
	}
	f.Pushed[localDir] = true
	return nil
}

func (f *FakeDoltCLI) PushBranch(localDir, branch string, _ bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	call := fmt.Sprintf("PushBranch(%s, %s)", localDir, branch)
	f.Calls = append(f.Calls, call)
	if f.Log != nil {
		f.Log.Record(call)
	}
	if f.PushErr != nil {
		return f.PushErr
	}
	f.Pushed[fmt.Sprintf("%s:%s", localDir, branch)] = true
	return nil
}

func (f *FakeDoltCLI) CheckoutBranch(localDir, branch string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	call := fmt.Sprintf("CheckoutBranch(%s, %s)", localDir, branch)
	f.Calls = append(f.Calls, call)
	if f.Log != nil {
		f.Log.Record(call)
	}
	f.Branches[fmt.Sprintf("%s->%s", localDir, branch)] = true
	return nil
}

func (f *FakeDoltCLI) CheckoutMain(localDir string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	call := fmt.Sprintf("CheckoutMain(%s)", localDir)
	f.Calls = append(f.Calls, call)
	if f.Log != nil {
		f.Log.Record(call)
	}
	return nil
}

func (f *FakeDoltCLI) AddUpstreamRemote(localDir, remoteURL string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	call := fmt.Sprintf("AddUpstreamRemote(%s, %s)", localDir, remoteURL)
	f.Calls = append(f.Calls, call)
	if f.Log != nil {
		f.Log.Record(call)
	}
	if f.RemoteErr != nil {
		return f.RemoteErr
	}
	f.Remotes[fmt.Sprintf("%s->%s", localDir, remoteURL)] = true
	return nil
}

// FakeConfigStore is a test double for ConfigStore.
type FakeConfigStore struct {
	mu      sync.Mutex
	Configs map[string]*Config // upstream path -> Config

	LoadErr   error
	SaveErr   error
	DeleteErr error
	ListErr   error
}

func NewFakeConfigStore() *FakeConfigStore {
	return &FakeConfigStore{
		Configs: make(map[string]*Config),
	}
}

func (f *FakeConfigStore) Load(upstream string) (*Config, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.LoadErr != nil {
		return nil, f.LoadErr
	}
	cfg, ok := f.Configs[upstream]
	if !ok {
		return nil, ErrNotJoined
	}
	return cfg, nil
}

func (f *FakeConfigStore) Save(cfg *Config) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.SaveErr != nil {
		return f.SaveErr
	}
	f.Configs[cfg.Upstream] = cfg
	return nil
}

func (f *FakeConfigStore) Delete(upstream string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.DeleteErr != nil {
		return f.DeleteErr
	}
	if _, ok := f.Configs[upstream]; !ok {
		return fmt.Errorf("%w: %s", ErrNotJoined, upstream)
	}
	delete(f.Configs, upstream)
	return nil
}

func (f *FakeConfigStore) List() ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.ListErr != nil {
		return nil, f.ListErr
	}
	var upstreams []string
	for k := range f.Configs {
		upstreams = append(upstreams, k)
	}
	sort.Strings(upstreams)
	return upstreams, nil
}
