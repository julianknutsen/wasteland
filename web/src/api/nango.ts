import Nango from "@nangohq/frontend";

/** Create a Nango frontend SDK instance for a connect session token. */
export function initNango(connectSessionToken: string): Nango {
  return new Nango({ connectSessionToken });
}

/**
 * Connect a DoltHub account via Nango. The user provides their API key,
 * which Nango stores encrypted — the app server never sees it directly.
 *
 * With connect session tokens the end-user identity is already embedded in
 * the session, so connectionId is not needed as a separate argument.
 */
export async function connectDoltHub(nango: Nango, integrationId: string, apiKey: string) {
  return nango.auth(integrationId, {
    credentials: { apiKey },
  });
}
