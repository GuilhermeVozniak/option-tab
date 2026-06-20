const STYLES = [
  { name: "Thumbnails", desc: "Live previews of every window, auto-sized to fit." },
  { name: "App Icons", desc: "A clean, dock-like row of large app icons." },
  { name: "Titles", desc: "A compact, text-only list for keyboard speed." },
];

const FEATURES = [
  {
    title: "Window-level switching",
    body: "See and pick individual windows across every app — not just one icon per app.",
  },
  {
    title: "Fuzzy search",
    body: "Just start typing to filter by window title or app name. No scrolling.",
  },
  {
    title: "Up to 9 shortcuts",
    body: "Independent chords, each with its own filter scope and visual style.",
  },
  {
    title: "Auto-sizing",
    body: "Thumbnails scale to the number of windows so the switcher stays readable.",
  },
  {
    title: "Spaces & multi-monitor",
    body: "Filter by active space, all spaces, the active screen, or the screen under your cursor.",
  },
  {
    title: "Window controls",
    body: "Close, minimize, hide, or quit straight from the switcher — hover and click.",
  },
  {
    title: "Deep filters",
    body: "Show or hide minimized windows, hidden apps, fullscreen windows, and blacklist apps.",
  },
  {
    title: "Made to feel native",
    body: "Frameless, translucent overlay, light/dark themes, custom accent color, and start-at-login.",
  },
];

export function Features() {
  return (
    <>
      <section className="styles" id="features">
        <h2>Three ways to switch</h2>
        <div className="style-grid">
          {STYLES.map((s) => (
            <div className="style-card" key={s.name}>
              <h3>{s.name}</h3>
              <p>{s.desc}</p>
            </div>
          ))}
        </div>
      </section>

      <section className="features">
        <h2>Everything AltTab does — including the paid features</h2>
        <div className="feature-grid">
          {FEATURES.map((f) => (
            <div className="feature-card" key={f.title}>
              <h3>{f.title}</h3>
              <p>{f.body}</p>
            </div>
          ))}
        </div>
      </section>
    </>
  );
}
