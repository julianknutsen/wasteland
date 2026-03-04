package hosted

import (
	"fmt"
	"log/slog"

	"github.com/julianknutsen/wasteland/internal/backend"
	"github.com/julianknutsen/wasteland/internal/commons"
	"github.com/julianknutsen/wasteland/internal/federation"
	"github.com/julianknutsen/wasteland/internal/remote"
)

// ForkRegistrar creates the DoltHub fork and registers the rig.
// Returns "" on success, or a warning string on failure.
type ForkRegistrar interface {
	EnsureForkAndRegister(apiKey, upstream, forkOrg, forkDB, rigHandle, displayName, email string) string
}

// DoltHubForkRegistrar is the production implementation of ForkRegistrar.
type DoltHubForkRegistrar struct{}

// EnsureForkAndRegister forks the upstream database and registers the rig.
// All steps are idempotent. Returns "" on success or a warning message.
func (d *DoltHubForkRegistrar) EnsureForkAndRegister(apiKey, upstream, forkOrg, forkDB, rigHandle, displayName, email string) string {
	if apiKey == "" {
		return "no API key available — fork and registration skipped"
	}

	upstreamOrg, upstreamDB, err := federation.ParseUpstream(upstream)
	if err != nil {
		return fmt.Sprintf("invalid upstream %q: %v", upstream, err)
	}

	provider := remote.NewDoltHubProvider(apiKey)

	// 1. Fork (idempotent — "already exists" is silent success).
	if err := provider.Fork(upstreamOrg, upstreamDB, forkOrg); err != nil {
		return fmt.Sprintf("fork failed: %v", err)
	}

	// 2. Register rig on a branch via the DoltHub SQL API.
	db := backend.NewRemoteDB(apiKey, upstreamOrg, upstreamDB, forkOrg, forkDB, federation.ModePR)
	branch := fmt.Sprintf("wl/register/%s", rigHandle)
	regSQL := commons.BuildRegistrationSQL(rigHandle, forkOrg, displayName, email, "hosted")
	if err := db.Exec(branch, "", false, regSQL); err != nil {
		return fmt.Sprintf("rig registration failed: %v", err)
	}

	// 3. Open PR (best-effort).
	title := fmt.Sprintf("Register rig: %s", rigHandle)
	body := fmt.Sprintf("Register rig **%s** (%s) in the commons.", rigHandle, displayName)
	if _, err := provider.CreatePR(forkOrg, upstreamOrg, upstreamDB, branch, title, body); err != nil {
		slog.Warn("fork registrar: PR creation failed", "error", err, "handle", rigHandle)
	}

	return ""
}
