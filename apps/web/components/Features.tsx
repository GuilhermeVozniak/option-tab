import { Card, CardDescription, CardTitle } from "@/components/ui/card";

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
      <section className="border-t border-white/10 py-14" id="features">
        <h2 className="m-0 mb-10 text-center text-[clamp(28px,4vw,40px)] font-bold tracking-tight">
          Three ways to switch
        </h2>
        <div className="grid grid-cols-[repeat(auto-fit,minmax(240px,1fr))] gap-5">
          {STYLES.map((s) => (
            <Card className="p-7 text-center" key={s.name}>
              <CardTitle className="mb-2 text-[22px]">{s.name}</CardTitle>
              <CardDescription>{s.desc}</CardDescription>
            </Card>
          ))}
        </div>
      </section>

      <section className="border-t border-white/10 py-14">
        <h2 className="m-0 mb-10 text-center text-[clamp(28px,4vw,40px)] font-bold tracking-tight">
          Everything AltTab does — including the paid features
        </h2>
        <div className="grid grid-cols-[repeat(auto-fit,minmax(260px,1fr))] gap-4">
          {FEATURES.map((f) => (
            <Card className="p-5" key={f.title}>
              <CardTitle className="mb-2 text-base">{f.title}</CardTitle>
              <CardDescription className="text-sm">{f.body}</CardDescription>
            </Card>
          ))}
        </div>
      </section>
    </>
  );
}
