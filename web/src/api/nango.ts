import Nango from "@nangohq/frontend";

let nangoInstance: Nango | null = null;

/** Initialize or return the Nango frontend SDK instance. */
export function initNango(publicKey: string): Nango {
  if (!nangoInstance) {
    nangoInstance = new Nango({ publicKey });
  }
  return nangoInstance;
}

/**
 * Connect a DoltHub account via Nango. The user provides their API key,
 * which Nango stores encrypted — the app server never sees it directly.
 */
export async function connectDoltHub(nango: Nango, integrationId: string, connectionId: string, apiKey: string) {
  return nango.auth(integrationId, connectionId, {
    credentials: { apiKey },
  });
}
