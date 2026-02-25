import { NavLink, Outlet } from 'react-router-dom';
import { ayu } from '../styles/theme';

const linkStyle = (isActive: boolean): React.CSSProperties => ({
  padding: '6px 12px',
  borderRadius: '4px',
  color: isActive ? ayu.accent : ayu.dim,
  background: isActive ? ayu.surface : 'transparent',
  textDecoration: 'none',
  fontWeight: isActive ? 600 : 400,
});

export function Layout() {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
      <nav
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: '4px',
          padding: '8px 16px',
          borderBottom: `1px solid ${ayu.border}`,
          background: ayu.surface,
        }}
      >
        <span style={{ fontWeight: 700, color: ayu.accent, marginRight: '16px' }}>wasteland</span>
        <NavLink to="/" end style={({ isActive }) => linkStyle(isActive)}>
          board
        </NavLink>
        <NavLink to="/me" style={({ isActive }) => linkStyle(isActive)}>
          me
        </NavLink>
      </nav>
      <main style={{ flex: 1, overflow: 'auto', padding: '16px' }}>
        <Outlet />
      </main>
    </div>
  );
}
