package hosted

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/julianknutsen/wasteland/internal/backend"
	"github.com/julianknutsen/wasteland/internal/federation"
	"github.com/julianknutsen/wasteland/internal/remote"
	"github.com/julianknutsen/wasteland/internal/sdk"
)

const cacheTTL = 5 * time.Minute

type cachedClient struct {
	client    *sdk.Client
	token     string
	expiresAt time.Time
}

// ClientResolver resolves per-user sdk.Clients from Nango credentials.
type ClientResolver struct {
	nango    *NangoClient
	sessions *SessionStore
	mu       sync.RWMutex
	cache    map[string]*cachedClient // sessionID -> cached client
}

// NewClientResolver creates a ClientResolver.
func NewClientResolver(nango *NangoClient, sessions *SessionStore) *ClientResolver {
	return &ClientResolver{
		nango:    nango,
		sessions: sessions,
		cache:    make(map[string]*cachedClient),
	}
}

// Resolve builds or returns a cached sdk.Client for the given session.
func (cr *ClientResolver) Resolve(session *UserSession) (*sdk.Client, error) {
	// Check cache first.
	cr.mu.RLock()
	if cached, ok := cr.cache[session.ID]; ok && time.Now().Before(cached.expiresAt) {
		cr.mu.RUnlock()
		return cached.client, nil
	}
	cr.mu.RUnlock()

	// Fetch fresh credentials + config from Nango.
	token, userCfg, err := cr.nango.GetConnection(session.ConnectionID)
	if err != nil {
		return nil, fmt.Errorf("resolving credentials: %w", err)
	}
	if token == "" {
		return nil, fmt.Errorf("no API token found for connection %s", session.ConnectionID)
	}
	if userCfg == nil {
		return nil, fmt.Errorf("no user config found for connection %s", session.ConnectionID)
	}

	// Check if cached client has the same token (just needs TTL refresh).
	cr.mu.RLock()
	if cached, ok := cr.cache[session.ID]; ok && cached.token == token {
		cached.expiresAt = time.Now().Add(cacheTTL)
		cr.mu.RUnlock()
		return cached.client, nil
	}
	cr.mu.RUnlock()

	// Build a new client.
	client, err := cr.buildClient(token, userCfg, session.ConnectionID)
	if err != nil {
		return nil, err
	}

	cr.mu.Lock()
	cr.cache[session.ID] = &cachedClient{
		client:    client,
		token:     token,
		expiresAt: time.Now().Add(cacheTTL),
	}
	cr.mu.Unlock()

	return client, nil
}

func (cr *ClientResolver) buildClient(token string, cfg *UserConfig, connectionID string) (*sdk.Client, error) {
	upOrg, upDB, err := federation.ParseUpstream(cfg.Upstream)
	if err != nil {
		return nil, fmt.Errorf("parsing upstream %q: %w", cfg.Upstream, err)
	}

	mode := cfg.Mode
	if mode == "" {
		mode = "wild-west"
	}

	db := backend.NewRemoteDB(token, upOrg, upDB, cfg.ForkOrg, cfg.ForkDB, mode)

	provider := remote.NewDoltHubProvider(token)

	branchURL := func(branch string) string {
		return fmt.Sprintf("https://www.dolthub.com/repositories/%s/%s/data/%s",
			cfg.ForkOrg, cfg.ForkDB, strings.ReplaceAll(branch, "/", "%2F"))
	}

	client := sdk.New(sdk.ClientConfig{
		DB:        db,
		RigHandle: cfg.RigHandle,
		Mode:      mode,
		LoadDiff: func(branch string) (string, error) {
			return db.Diff(branch)
		},
		CreatePR: func(branch string) (string, error) {
			prURL, err := provider.CreatePR(cfg.ForkOrg, upOrg, upDB, branch, "", "")
			if err != nil && strings.Contains(err.Error(), "already exists") {
				existingURL, existingID := provider.FindPR(upOrg, upDB, cfg.ForkOrg, branch)
				if existingID != "" {
					return existingURL, nil
				}
			}
			return prURL, err
		},
		CheckPR: func(branch string) string {
			url, _ := provider.FindPR(upOrg, upDB, cfg.ForkOrg, branch)
			return url
		},
		ClosePR: func(branch string) error {
			_, prID := provider.FindPR(upOrg, upDB, cfg.ForkOrg, branch)
			if prID == "" {
				return nil
			}
			return provider.ClosePR(upOrg, upDB, prID)
		},
		ListPendingItems: func() (map[string]bool, error) {
			return provider.ListPendingWantedIDs(upOrg, upDB)
		},
		BranchURL: branchURL,
		SaveConfig: func(mode string, _ bool) error {
			newCfg := *cfg
			newCfg.Mode = mode
			return cr.nango.SetMetadata(connectionID, &newCfg)
		},
	})

	return client, nil
}
