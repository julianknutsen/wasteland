import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { toast } from "sonner";
import { authStatus, nangoKey, notifyConnect } from "../api/client";
import { connectDoltHub, initNango } from "../api/nango";
import styles from "./ConnectPage.module.css";

export function ConnectPage() {
  const navigate = useNavigate();
  const [step, setStep] = useState<"identity" | "connect">("identity");
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);

  // Identity form state.
  const [rigHandle, setRigHandle] = useState("");
  const [forkOrg, setForkOrg] = useState("");
  const [forkDB, setForkDB] = useState("wl-commons");
  const [upstream, setUpstream] = useState("");

  // DoltHub token step.
  const [apiToken, setApiToken] = useState("");

  // Nango config from server.
  const [publicKey, setPublicKey] = useState("");
  const [integrationId, setIntegrationId] = useState("");

  // Check if already authenticated on mount.
  useEffect(() => {
    (async () => {
      try {
        const status = await authStatus();
        if (status.authenticated && status.connected) {
          navigate("/", { replace: true });
          return;
        }
        const key = await nangoKey();
        setPublicKey(key.public_key);
        setIntegrationId(key.integration_id);
      } catch {
        // Server may not be in hosted mode — nango-key 404 is expected.
      } finally {
        setLoading(false);
      }
    })();
  }, [navigate]);

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
      const connectionId = rigHandle.trim();
      const nango = initNango(publicKey);

      // Store the token in Nango.
      await connectDoltHub(nango, integrationId, connectionId, apiToken.trim());

      // Notify the backend to create the session and store config.
      await notifyConnect({
        connection_id: connectionId,
        rig_handle: rigHandle.trim(),
        fork_org: forkOrg.trim(),
        fork_db: forkDB.trim(),
        upstream: upstream.trim(),
      });

      toast.success("Connected to DoltHub");
      navigate("/", { replace: true });
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Connection failed");
    } finally {
      setSubmitting(false);
    }
  };

  if (loading) return <p className={styles.loadingText}>Loading...</p>;

  return (
    <div className={styles.page}>
      <h2 className={styles.heading}>Connect to Wasteland</h2>

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
              Paste your DoltHub API token. It will be stored securely via Nango and never saved on this server.
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
