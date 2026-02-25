import styles from "./ShortcutHelp.module.css";

interface ShortcutHelpProps {
  open: boolean;
  onClose: () => void;
}

const shortcuts = [
  {
    title: "Navigation",
    items: [
      { label: "Board", key: "g b" },
      { label: "Dashboard", key: "g d" },
      { label: "Settings", key: "g s" },
    ],
  },
  {
    title: "Actions",
    items: [
      { label: "Command Palette", key: "\u2318K" },
      { label: "Create Item", key: "c" },
      { label: "Shortcut Help", key: "?" },
    ],
  },
  {
    title: "Browse List",
    items: [
      { label: "Move Down", key: "j" },
      { label: "Move Up", key: "k" },
      { label: "Open Item", key: "Enter" },
      { label: "Focus Search", key: "/" },
    ],
  },
];

export function ShortcutHelp({ open, onClose }: ShortcutHelpProps) {
  if (!open) return null;

  return (
    <div className={styles.overlay} onClick={onClose}>
      <div className={styles.dialog} onClick={(e) => e.stopPropagation()}>
        <h2 className={styles.title}>Keyboard Shortcuts</h2>
        {shortcuts.map((section) => (
          <div key={section.title} className={styles.section}>
            <h3 className={styles.sectionTitle}>{section.title}</h3>
            {section.items.map((item) => (
              <div key={item.key} className={styles.row}>
                <span className={styles.label}>{item.label}</span>
                <span className={styles.key}>{item.key}</span>
              </div>
            ))}
          </div>
        ))}
        <button type="button" className={styles.closeBtn} onClick={onClose}>
          Close
        </button>
      </div>
    </div>
  );
}
