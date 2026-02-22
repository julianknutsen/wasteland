package federation

import (
	"fmt"
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
	mu      sync.Mutex
	Forked  map[string]bool // "fromOrg/fromDB->toOrg" => true
	Calls   []string
	Log     *CallLog // shared ordered log (optional)
	ForkErr error
	BaseURL string // returned by DatabaseURL (default "https://fake-remote")
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

func (f *FakeProvider) Type() string { return "fake" }

// FakeDoltCLI is a test double for DoltCLI.
type FakeDoltCLI struct {
	mu         sync.Mutex
	Cloned     map[string]bool // "remoteURL -> targetDir"
	Registered map[string]bool // "handle"
	Pushed     map[string]bool // "localDir"
	Remotes    map[string]bool // "localDir -> remoteURL"
	Calls      []string
	Log        *CallLog // shared ordered log (optional)

	CloneErr    error
	RegisterErr error
	PushErr     error
	RemoteErr   error
}

func NewFakeDoltCLI() *FakeDoltCLI {
	return &FakeDoltCLI{
		Cloned:     make(map[string]bool),
		Registered: make(map[string]bool),
		Pushed:     make(map[string]bool),
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
		return f.CloneErr
	}
	f.Cloned[fmt.Sprintf("%s->%s", remoteURL, targetDir)] = true
	return nil
}

func (f *FakeDoltCLI) RegisterRig(localDir, handle, dolthubOrg, displayName, ownerEmail, version string) error {
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
	mu     sync.Mutex
	Config *Config // stored config (nil = not joined)

	LoadErr error
	SaveErr error
}

func NewFakeConfigStore() *FakeConfigStore {
	return &FakeConfigStore{}
}

func (f *FakeConfigStore) Load() (*Config, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.LoadErr != nil {
		return nil, f.LoadErr
	}
	if f.Config == nil {
		return nil, ErrNotJoined
	}
	return f.Config, nil
}

func (f *FakeConfigStore) Save(cfg *Config) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.SaveErr != nil {
		return f.SaveErr
	}
	f.Config = cfg
	return nil
}
