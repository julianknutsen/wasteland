import { useEffect, useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { toast } from "sonner";
import { authStatus, connectSession, joinWasteland, notifyConnect } from "../api/client";
import { connectDoltHub, initNango } from "../api/nango";
import { useWasteland } from "../context/WastelandContext";
import styles from "./ConnectPage.module.css";

export function ConnectPage() {
  const navigate = useNavigate();
  const location = useLocation();
  const { refresh } = useWasteland();
  const [step, setStep] = useState<"identity" | "connect" | "join">("identity");
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);

  // Identity form state.
  const [rigHandle, setRigHandle] = useState("");
  const [forkOrg, setForkOrg] = useState("");
  const [forkDB, setForkDB] = useState("wl-commons");
  const [upstream, setUpstream] = useState("hop/wl-commons");

  // DoltHub token step.
  const [apiToken, setApiToken] = useState("");

  // Check if already authenticated on mount.
  useEffect(() => {
    (async () => {
      try {
        const status = await authStatus();
        if (status.authenticated && status.connected) {
          // If arriving at /join, show the simplified join form.
          if (location.pathname === "/join") {
            setStep("join");
          } else {
            navigate("/", { replace: true });
            return;
          }
        }
      } catch {
        // Server may not be in hosted mode -- status 404 is expected.
      } finally {
        setLoading(false);
      }
    })();
  }, [navigate, location.pathname]);

  const handleIdentitySubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!rigHandle.trim() || !forkOrg.trim() || !forkDB.trim() || !upstream.trim()) {
      toast.error("All fields are required");
      return;
    }
    setStep("connect");
  };

  const handleConnect = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!apiToken.trim()) {
      toast.error("DoltHub API token is required");
      return;
    }

    setSubmitting(true);
    try {
      const endUserId = rigHandle.trim();

      // Create a connect session token and store the token in Nango.
      const session = await connectSession(endUserId);
      const nango = initNango(session.token);
      const authResult = await connectDoltHub(nango, session.integration_id, apiToken.trim());

      // Notify the backend to create the session and store config.
      // Use the actual connectionId assigned by Nango during auth.
      await notifyConnect({
        connection_id: authResult.connectionId,
        rig_handle: rigHandle.trim(),
        fork_org: forkOrg.trim(),
        fork_db: forkDB.trim(),
        upstream: upstream.trim(),
      });

      await refresh();
      toast.success("Connected to DoltHub");
      navigate("/", { replace: true });
    } catch (err) {
      toast.error(err instanceof Error && err.message ? err.message : "Connection failed");
    } finally {
      setSubmitting(false);
    }
  };

  const handleJoin = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!forkOrg.trim() || !forkDB.trim() || !upstream.trim()) {
      toast.error("Fork org, fork DB, and upstream are required");
      return;
    }

    setSubmitting(true);
    try {
      await joinWasteland({
        fork_org: forkOrg.trim(),
        fork_db: forkDB.trim(),
        upstream: upstream.trim(),
      });

      await refresh();
      toast.success("Joined wasteland");
      navigate("/", { replace: true });
    } catch (err) {
      toast.error(err instanceof Error && err.message ? err.message : "Join failed");
    } finally {
      setSubmitting(false);
    }
  };

  if (loading) return <p className={styles.loadingText}>Loading...</p>;

  return (
    <div className={styles.page}>
      <h2 className={styles.heading}>{step === "join" ? "Join a Wasteland" : "Connect to Wasteland"}</h2>

      {step === "join" && (
        <form onSubmit={handleJoin}>
          <div className={styles.section}>
            <h3 className={styles.sectionTitle}>Wasteland Details</h3>

            <label className={styles.fieldLabel}>
              Upstream
              <input
                className={styles.input}
                type="text"
                value={upstream}
                onChange={(e) => setUpstream(e.target.value)}
                placeholder="org/wl-commons"
              />
            </label>

            <label className={styles.fieldLabel}>
              Fork Org
              <input
                className={styles.input}
                type="text"
                value={forkOrg}
                onChange={(e) => setForkOrg(e.target.value)}
                placeholder="your-dolthub-org"
              />
            </label>

            <label className={styles.fieldLabel}>
              Fork DB
              <input
                className={styles.input}
                type="text"
                value={forkDB}
                onChange={(e) => setForkDB(e.target.value)}
                placeholder="wl-commons"
              />
            </label>
          </div>

          <div className={styles.actions}>
            <button type="button" className={styles.secondaryBtn} onClick={() => navigate("/settings")}>
              Cancel
            </button>
            <button type="submit" className={styles.primaryBtn} disabled={submitting}>
              {submitting ? "Joining..." : "Join"}
            </button>
          </div>
        </form>
      )}

      {step === "identity" && (
        <form onSubmit={handleIdentitySubmit}>
          <div className={styles.section}>
            <h3 className={styles.sectionTitle}>Identity</h3>

            <label className={styles.fieldLabel}>
              Rig Handle
              <input
                className={styles.input}
                type="text"
                value={rigHandle}
                onChange={(e) => setRigHandle(e.target.value)}
                placeholder="your-handle"
              />
            </label>

            <label className={styles.fieldLabel}>
              Fork Org
              <input
                className={styles.input}
                type="text"
                value={forkOrg}
                onChange={(e) => setForkOrg(e.target.value)}
                placeholder="your-dolthub-org"
              />
            </label>

            <label className={styles.fieldLabel}>
              Fork DB
              <input
                className={styles.input}
                type="text"
                value={forkDB}
                onChange={(e) => setForkDB(e.target.value)}
                placeholder="wl-commons"
              />
            </label>

            <label className={styles.fieldLabel}>
              Upstream
              <input
                className={styles.input}
                type="text"
                value={upstream}
                onChange={(e) => setUpstream(e.target.value)}
                placeholder="org/wl-commons"
              />
            </label>
          </div>

          <div className={styles.actions}>
            <button type="submit" className={styles.primaryBtn}>
              Next
            </button>
          </div>
        </form>
      )}

      {step === "connect" && (
        <form onSubmit={handleConnect}>
          <div className={styles.section}>
            <h3 className={styles.sectionTitle}>Connect DoltHub</h3>
            <p className={styles.hint}>
              Your token is sent directly to{" "}
              <a href="https://www.nango.dev" target="_blank" rel="noopener noreferrer" className={styles.link}>
                Nango
              </a>
              , a third-party credentials vault. It is encrypted at rest and never touches the Wasteland server. DoltHub
              API calls are proxied through Nango, which injects your token — our server never sees it.
            </p>

            <label className={styles.fieldLabel}>
              DoltHub API Token
              <input
                className={styles.input}
                type="password"
                value={apiToken}
                onChange={(e) => setApiToken(e.target.value)}
                placeholder="your-dolthub-api-token"
              />
            </label>
          </div>

          <div className={styles.actions}>
            <button type="button" className={styles.secondaryBtn} onClick={() => setStep("identity")}>
              Back
            </button>
            <button type="submit" className={styles.primaryBtn} disabled={submitting}>
              {submitting ? "Connecting..." : "Connect"}
            </button>
          </div>
        </form>
      )}
    </div>
  );
}
