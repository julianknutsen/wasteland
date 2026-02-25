import { useEffect, useRef } from "react";
import { useNavigate } from "react-router-dom";

interface ShortcutHandlers {
  onCreateItem?: () => void;
  onTogglePalette: () => void;
  onToggleHelp: () => void;
}

export function useGlobalShortcuts({ onCreateItem, onTogglePalette, onToggleHelp }: ShortcutHandlers) {
  const navigate = useNavigate();
  const pendingG = useRef(false);
  const timerRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      const target = e.target as HTMLElement;
      const inInput =
        target.tagName === "INPUT" ||
        target.tagName === "TEXTAREA" ||
        target.tagName === "SELECT" ||
        target.isContentEditable;

      // Cmd+K always works
      if ((e.metaKey || e.ctrlKey) && e.key === "k") {
        e.preventDefault();
        onTogglePalette();
        return;
      }

      // Skip other shortcuts when in an input
      if (inInput) return;

      // g-prefixed navigation
      if (pendingG.current) {
        pendingG.current = false;
        if (timerRef.current) clearTimeout(timerRef.current);
        switch (e.key) {
          case "b":
            navigate("/");
            return;
          case "d":
            navigate("/me");
            return;
          case "s":
            navigate("/settings");
            return;
        }
        return;
      }

      if (e.key === "g") {
        pendingG.current = true;
        timerRef.current = setTimeout(() => {
          pendingG.current = false;
        }, 500);
        return;
      }

      if (e.key === "c" && onCreateItem) {
        onCreateItem();
        return;
      }

      if (e.key === "?") {
        onToggleHelp();
        return;
      }
    };

    window.addEventListener("keydown", handler);
    return () => {
      window.removeEventListener("keydown", handler);
      if (timerRef.current) clearTimeout(timerRef.current);
    };
  }, [navigate, onCreateItem, onTogglePalette, onToggleHelp]);
}
