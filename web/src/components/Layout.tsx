import { NavLink, Outlet } from 'react-router-dom';
import { ayu } from '../styles/theme';

const linkStyle = (isActive: boolean): React.CSSProperties => ({
  padding: '6px 14px',
  borderRadius: '4px',
  color: isActive ? ayu.fgLight : ayu.fgMuted,
  background: isActive ? ayu.surfaceDark : 'transparent',
  textDecoration: 'none',
  fontWeight: isActive ? 700 : 400,
  fontFamily: "'Cinzel', 'Times New Roman', serif",
  fontSize: '13px',
  letterSpacing: '0.05em',
  textTransform: 'uppercase',
});

export function Layout() {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
      <nav
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: '4px',
          padding: '10px 20px',
          borderBottom: `2px solid ${ayu.border}`,
          background: ayu.bgDark,
        }}
      >
        <span
          style={{
            fontFamily: "'Cinzel', 'Times New Roman', serif",
            fontWeight: 700,
            fontSize: '18px',
            color: ayu.accentLight,
            marginRight: '20px',
            letterSpacing: '0.1em',
            textTransform: 'uppercase',
          }}
        >
          wasteland
        </span>
        <NavLink to="/" end style={({ isActive }) => linkStyle(isActive)}>
          board
        </NavLink>
        <NavLink to="/me" style={({ isActive }) => linkStyle(isActive)}>
          me
        </NavLink>
      </nav>
      <main
        style={{
          flex: 1,
          overflow: 'auto',
          padding: '20px',
          maxWidth: '1200px',
          width: '100%',
          margin: '0 auto',
        }}
      >
        <Outlet />
      </main>
    </div>
  );
}
