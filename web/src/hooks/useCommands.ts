import { createContext, useContext, useCallback, useRef } from 'react';

export interface Command {
  id: string;
  label: string;
  group: string;
  shortcut?: string;
  action: () => void;
}

interface CommandsContextValue {
  commands: Command[];
  register: (cmds: Command[]) => () => void;
}

export const CommandsContext = createContext<CommandsContextValue>({
  commands: [],
  register: () => () => {},
});

export function useCommands() {
  return useContext(CommandsContext);
}

export function useCommandRegistry() {
  const commandsRef = useRef<Map<string, Command[]>>(new Map());
  const listenersRef = useRef<Set<() => void>>(new Set());
  const snapshotRef = useRef<Command[]>([]);

  const rebuildSnapshot = useCallback(() => {
    const all: Command[] = [];
    for (const cmds of commandsRef.current.values()) {
      all.push(...cmds);
    }
    snapshotRef.current = all;
    for (const listener of listenersRef.current) {
      listener();
    }
  }, []);

  const register = useCallback((cmds: Command[]) => {
    const key = Math.random().toString(36).slice(2);
    commandsRef.current.set(key, cmds);
    rebuildSnapshot();
    return () => {
      commandsRef.current.delete(key);
      rebuildSnapshot();
    };
  }, [rebuildSnapshot]);

  const getCommands = useCallback(() => snapshotRef.current, []);

  const subscribe = useCallback((listener: () => void) => {
    listenersRef.current.add(listener);
    return () => { listenersRef.current.delete(listener); };
  }, []);

  return { register, getCommands, subscribe };
}
