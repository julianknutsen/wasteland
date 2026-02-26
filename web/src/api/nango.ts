import Nango from "@nangohq/frontend";

/** Create a Nango frontend SDK instance for a connect session token. */
export function initNango(connectSessionToken: string): Nango {
  return new Nango({ connectSessionToken });
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
