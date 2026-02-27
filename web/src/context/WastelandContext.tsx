import type { ReactNode } from "react";
import { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";
import { authStatus, setActiveUpstream } from "../api/client";
import type { WastelandConfig } from "../api/types";

interface WastelandContextValue {
  wastelands: WastelandConfig[];
  active: string | null;
  switchTo: (upstream: string) => void;
  refresh: () => Promise<void>;
}

const WastelandContext = createContext<WastelandContextValue>({
  wastelands: [],
  active: null,
  switchTo: () => {},
  refresh: async () => {},
});

const STORAGE_KEY = "wl_active";

export function WastelandProvider({ children }: { children: ReactNode }) {
  const [wastelands, setWastelands] = useState<WastelandConfig[]>([]);
  const [active, setActive] = useState<string | null>(null);

  const applyActive = useCallback((upstream: string | null) => {
    setActive(upstream);
    setActiveUpstream(upstream);
    if (upstream) {
      localStorage.setItem(STORAGE_KEY, upstream);
    } else {
      localStorage.removeItem(STORAGE_KEY);
    }
  }, []);

  const refresh = useCallback(async () => {
    try {
      const status = await authStatus();
      const wls = status.wastelands ?? [];
      setWastelands(wls);

      if (wls.length === 0) {
        applyActive(null);
        return;
      }

      // Pick active: prefer stored, fall back to first.
      const stored = localStorage.getItem(STORAGE_KEY);
      const match = stored && wls.some((w) => w.upstream === stored);
      applyActive(match ? stored : wls[0].upstream);
    } catch {
      // Not in hosted mode or server not running — no wastelands.
    }
  }, [applyActive]);

  useEffect(() => {
    refresh();
  }, [refresh]);

  const switchTo = useCallback(
    (upstream: string) => {
      applyActive(upstream);
    },
    [applyActive],
  );

  const value = useMemo(() => ({ wastelands, active, switchTo, refresh }), [wastelands, active, switchTo, refresh]);

  return <WastelandContext.Provider value={value}>{children}</WastelandContext.Provider>;
}

export function useWasteland(): WastelandContextValue {
  return useContext(WastelandContext);
}
