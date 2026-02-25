import { Command } from "cmdk";
import { useSyncExternalStore } from "react";
import { useCommands } from "../hooks/useCommands";
import styles from "./CommandPalette.module.css";

interface CommandPaletteProps {
  open: boolean;
  onClose: () => void;
}

export function CommandPalette({ open, onClose }: CommandPaletteProps) {
  const { commands } = useCommands();

  if (!open) return null;

  const groups = new Map<string, typeof commands>();
  for (const cmd of commands) {
    const list = groups.get(cmd.group) ?? [];
    list.push(cmd);
    groups.set(cmd.group, list);
  }

  return (
    <div className={styles.overlay} onClick={onClose}>
      <div className={styles.palette} onClick={(e) => e.stopPropagation()}>
        <Command label="Command Palette">
          <Command.Input className={styles.input} placeholder="Type a command..." autoFocus />
          <Command.List className={styles.list}>
            <Command.Empty className={styles.empty}>No results found.</Command.Empty>
            {[...groups.entries()].map(([group, cmds]) => (
              <Command.Group key={group} heading={group} className={styles.group}>
                {cmds.map((cmd) => (
                  <Command.Item
                    key={cmd.id}
                    className={styles.item}
                    onSelect={() => {
                      cmd.action();
                      onClose();
                    }}
                  >
                    <span>{cmd.label}</span>
                    {cmd.shortcut && <span className={styles.shortcut}>{cmd.shortcut}</span>}
                  </Command.Item>
                ))}
              </Command.Group>
            ))}
          </Command.List>
        </Command>
      </div>
    </div>
  );
}

export { useSyncExternalStore };
