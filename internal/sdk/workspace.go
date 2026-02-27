package sdk

import (
	"fmt"
	"sort"
	"sync"
)

// UpstreamInfo describes a joined wasteland.
type UpstreamInfo struct {
	Upstream string `json:"upstream"`
	ForkOrg  string `json:"fork_org"`
	ForkDB   string `json:"fork_db"`
	Mode     string `json:"mode"`
}

// Workspace holds multiple named Clients, one per upstream.
type Workspace struct {
	mu        sync.RWMutex
	clients   map[string]*Client
	info      map[string]UpstreamInfo
	rigHandle string
}

// NewWorkspace creates a Workspace for the given rig handle.
func NewWorkspace(rigHandle string) *Workspace {
	return &Workspace{
		clients:   make(map[string]*Client),
		info:      make(map[string]UpstreamInfo),
		rigHandle: rigHandle,
	}
}

// Add registers a client for the given upstream.
func (w *Workspace) Add(info UpstreamInfo, client *Client) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.clients[info.Upstream] = client
	w.info[info.Upstream] = info
}

// Remove unregisters the client for the given upstream.
func (w *Workspace) Remove(upstream string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.clients, upstream)
	delete(w.info, upstream)
}

// Client returns the client for the given upstream.
func (w *Workspace) Client(upstream string) (*Client, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	c, ok := w.clients[upstream]
	if !ok {
		return nil, fmt.Errorf("no client for upstream %q", upstream)
	}
	return c, nil
}

// Upstreams returns info about all registered upstreams, sorted by upstream name.
func (w *Workspace) Upstreams() []UpstreamInfo {
	w.mu.RLock()
	defer w.mu.RUnlock()
	result := make([]UpstreamInfo, 0, len(w.info))
	for _, info := range w.info {
		result = append(result, info)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Upstream < result[j].Upstream
	})
	return result
}

// RigHandle returns the rig handle for this workspace.
func (w *Workspace) RigHandle() string { return w.rigHandle }
