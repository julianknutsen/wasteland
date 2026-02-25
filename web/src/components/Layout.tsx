import { useState, useCallback, useMemo, useSyncExternalStore } from 'react';
import { NavLink, Outlet, useNavigate } from 'react-router-dom';
import { CommandsContext, useCommandRegistry } from '../hooks/useCommands';
import { useGlobalShortcuts } from '../hooks/useGlobalShortcuts';
import { CommandPalette } from './CommandPalette';
import { ShortcutHelp } from './ShortcutHelp';
import styles from './Layout.module.css';

export function Layout() {
  const [paletteOpen, setPaletteOpen] = useState(false);
  const [helpOpen, setHelpOpen] = useState(false);
  const navigate = useNavigate();

  const { register, getCommands, subscribe } = useCommandRegistry();
  const commands = useSyncExternalStore(subscribe, getCommands);

  const navCommands = useMemo(() => [
    { id: 'nav-board', label: 'Go to Board', group: 'Navigation', shortcut: 'g b', action: () => navigate('/') },
    { id: 'nav-dashboard', label: 'Go to Dashboard', group: 'Navigation', shortcut: 'g d', action: () => navigate('/me') },
    { id: 'nav-settings', label: 'Go to Settings', group: 'Navigation', shortcut: 'g s', action: () => navigate('/settings') },
  ], [navigate]);

  // Register navigation commands once
  useState(() => register(navCommands));

  const togglePalette = useCallback(() => setPaletteOpen((o) => !o), []);
  const toggleHelp = useCallback(() => setHelpOpen((o) => !o), []);

  useGlobalShortcuts({
    onTogglePalette: togglePalette,
    onToggleHelp: toggleHelp,
  });

  const contextValue = useMemo(() => ({ commands, register }), [commands, register]);

  return (
    <CommandsContext.Provider value={contextValue}>
      <div className={styles.layout}>
        <a href="#main-content" className="skip-link">Skip to content</a>
        <nav className={styles.nav} role="navigation" aria-label="Main navigation">
          <span className={styles.logo}>wasteland</span>
          <NavLink
            to="/"
            end
            className={({ isActive }) => isActive ? styles.navLinkActive : styles.navLink}
          >
            board
          </NavLink>
          <NavLink
            to="/me"
            className={({ isActive }) => isActive ? styles.navLinkActive : styles.navLink}
          >
            me
          </NavLink>
          <NavLink
            to="/settings"
            className={({ isActive }) => isActive ? styles.navLinkActive : styles.navLink}
          >
            settings
          </NavLink>
        </nav>
        <main id="main-content" className={styles.main}>
          <Outlet />
        </main>
      </div>

      <CommandPalette open={paletteOpen} onClose={() => setPaletteOpen(false)} />
      <ShortcutHelp open={helpOpen} onClose={() => setHelpOpen(false)} />
    </CommandsContext.Provider>
  );
}
