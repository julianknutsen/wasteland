package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os/exec"

	"github.com/julianknutsen/wasteland/internal/api"
	"github.com/julianknutsen/wasteland/internal/backend"
	"github.com/julianknutsen/wasteland/internal/commons"
	"github.com/julianknutsen/wasteland/internal/federation"
	"github.com/julianknutsen/wasteland/internal/sdk"
	"github.com/julianknutsen/wasteland/internal/style"
	"github.com/julianknutsen/wasteland/web"
	"github.com/spf13/cobra"
)

func newServeCmd(stdout, stderr io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the web UI server",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runServe(cmd, stdout, stderr)
		},
	}
	cmd.Flags().Int("port", 8999, "Port to listen on")
	cmd.Flags().Bool("dev", false, "Enable CORS for development (Vite proxy)")
	return cmd
}

func runServe(cmd *cobra.Command, stdout, stderr io.Writer) error {
	port, _ := cmd.Flags().GetInt("port")
	devMode, _ := cmd.Flags().GetBool("dev")

	cfg, err := resolveWasteland(cmd)
	if err != nil {
		return fmt.Errorf("loading wasteland config: %w", err)
	}

	var db commons.DB
	if cfg.ResolveBackend() == federation.BackendLocal {
		if err := requireDolt(); err != nil {
			return err
		}
		localDB := backend.NewLocalDB(cfg.LocalDir, cfg.ResolveMode())
		db = localDB

		sp := style.StartSpinner(stderr, "Syncing with upstream...")
		err = localDB.Sync()
		sp.Stop()
		if err != nil {
			return fmt.Errorf("syncing with upstream: %w", err)
		}

		if cfg.ResolveMode() == federation.ModePR {
			if err := localDB.PushMain(io.Discard); err != nil {
				fmt.Fprintf(stderr, "  warning: could not sync origin/main: %v\n", err)
			}
		}
	} else {
		upOrg, upDB, err := federation.ParseUpstream(cfg.Upstream)
		if err != nil {
			return fmt.Errorf("parsing upstream: %w", err)
		}
		remoteDB := backend.NewRemoteDB(commons.DoltHubToken(), upOrg, upDB, cfg.ForkOrg, cfg.ForkDB, cfg.ResolveMode())
		db = remoteDB

		sp := style.StartSpinner(stderr, "Syncing fork with upstream...")
		err = remoteDB.Sync()
		sp.Stop()
		if err != nil {
			fmt.Fprintf(stderr, "  warning: fork sync skipped: %v\n", err)
		}
	}

	// Build LoadDiff callback based on backend type.
	loadDiff := func(branch string) (string, error) {
		if cfg.ResolveBackend() != federation.BackendLocal {
			if rdb, ok := db.(*backend.RemoteDB); ok {
				return rdb.Diff(branch)
			}
			return "", fmt.Errorf("diff view requires local backend")
		}
		doltPath, err := exec.LookPath("dolt")
		if err != nil {
			return "", err
		}
		base := diffBase(cfg.LocalDir, doltPath)
		var buf bytes.Buffer
		if err := renderMarkdownDiff(&buf, cfg.LocalDir, doltPath, branch, base); err != nil {
			return "", err
		}
		return buf.String(), nil
	}

	client := sdk.New(sdk.ClientConfig{
		DB:        db,
		RigHandle: cfg.RigHandle,
		Mode:      cfg.ResolveMode(),
		Signing:   cfg.Signing,
		HopURI:    cfg.HopURI,
		SaveConfig: func(mode string, signing bool) error {
			store := federation.NewConfigStore()
			c, err := store.Load(cfg.Upstream)
			if err != nil {
				return err
			}
			c.Mode = mode
			c.Signing = signing
			return store.Save(c)
		},
		LoadDiff: loadDiff,
		CreatePR: func(branch string) (string, error) {
			if cfg.ResolveBackend() != federation.BackendLocal {
				return createPRForBranchRemote(cfg, branch)
			}
			return createPRForBranch(cfg, branch)
		},
		CheckPR: func(branch string) string {
			return checkPRForBranch(cfg, branch)
		},
		ListPendingItems: listPendingItemsFromPRs(cfg),
	})

	server := api.New(client)

	handler := api.SPAHandler(server, web.Assets)
	if devMode {
		handler = api.CORSMiddleware(handler)
	}

	addr := fmt.Sprintf(":%d", port)
	fmt.Fprintf(stdout, "Wasteland web UI listening on http://localhost%s\n", addr)
	return http.ListenAndServe(addr, handler) //nolint:gosec // bind addr is user-controlled via --port flag
}
