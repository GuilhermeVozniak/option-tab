import { DownloadButtons } from "../components/DownloadButtons";
import { Features } from "../components/Features";
import { PrimaryDownload } from "../components/PrimaryDownload";
import { buttonVariants } from "../components/ui/button";
import { latestReleaseUrl } from "../lib/download";

// SwitcherMock is the hero's signature element: the product itself, rendered
// as a frosted-glass switcher floating over the aurora. Purely decorative.
function SwitcherMock() {
  const tiles = [
    { name: "Editor", glyph: "E", hue: "from-indigo-400/50 to-indigo-600/30" },
    { name: "Browser", glyph: "B", hue: "from-cyan-400/50 to-cyan-600/30", selected: true },
    { name: "Terminal", glyph: "T", hue: "from-violet-400/50 to-violet-600/30" },
    { name: "Music", glyph: "M", hue: "from-fuchsia-400/50 to-fuchsia-600/30" },
  ];
  return (
    <div aria-hidden="true" className="mt-14 flex justify-center">
      <div className="inline-flex gap-3 rounded-3xl border border-white/15 bg-white/8 p-4 shadow-[inset_0_1px_0_rgba(255,255,255,0.2),0_32px_90px_-20px_rgba(0,0,0,0.8)] backdrop-blur-2xl">
        {tiles.map((tile) => (
          <div
            key={tile.name}
            className={`flex w-28 flex-col items-center gap-2 rounded-2xl border p-3 sm:w-32 ${
              tile.selected
                ? "border-cyan-300/60 bg-cyan-400/15 shadow-[inset_0_1px_0_rgba(255,255,255,0.25),0_12px_32px_-12px_rgba(34,211,238,0.8)]"
                : "border-white/10 bg-white/5"
            }`}
          >
            <div
              className={`flex size-12 items-center justify-center rounded-xl border border-white/25 bg-gradient-to-br text-lg font-bold text-white shadow-[inset_0_1px_0_rgba(255,255,255,0.35)] ${tile.hue}`}
            >
              {tile.glyph}
            </div>
            <span className="text-xs text-foreground/80">{tile.name}</span>
          </div>
        ))}
      </div>
    </div>
  );
}

export default function Home() {
  return (
    <main className="mx-auto max-w-[1080px] px-6">
      <section className="pb-16 pt-24 text-center">
        <span className="inline-block rounded-full border border-white/15 bg-white/8 px-4 py-1.5 text-sm text-muted-foreground shadow-[inset_0_1px_0_rgba(255,255,255,0.12)] backdrop-blur-md">
          100% free &amp; open-source — every Pro feature included
        </span>
        <h1 className="mx-auto mb-4 mt-7 bg-gradient-to-r from-white via-indigo-300 to-cyan-300 bg-clip-text text-[clamp(48px,9vw,88px)] font-bold leading-[1.02] tracking-tight text-transparent">
          Option Tab
        </h1>
        <p className="mx-auto mb-9 max-w-[640px] text-xl text-muted-foreground">
          The Windows-style <kbd>Alt</kbd>+<kbd>Tab</kbd> window switcher for macOS — switch by
          window, not just by app, with live previews, fuzzy search, and up to 9 custom shortcuts.
        </p>
        <div className="mb-5 flex flex-wrap items-center justify-center gap-3">
          <PrimaryDownload />
          <a className={buttonVariants({ variant: "glass", size: "lg" })} href={latestReleaseUrl()}>
            All releases
          </a>
        </div>
        <DownloadButtons />
        <p className="m-0 text-sm text-muted-foreground">
          macOS 13+ · Windows &amp; Linux builds available · no account, no paywall
        </p>
        <SwitcherMock />
      </section>

      <Features />

      <footer className="border-t border-white/10 py-12 pb-16 text-center text-muted-foreground">
        <p className="m-0 mb-2">
          Free forever under the Apache-2.0 license.{" "}
          <a
            className="text-cyan-300 no-underline hover:underline"
            href="https://github.com/GuilhermeVozniak/option-tab"
          >
            Source on GitHub
          </a>
          .
        </p>
        <p className="m-0 text-sm opacity-80">
          Inspired by AltTab — rebuilt in Go &amp; Wails, with the paid features free for everyone.
        </p>
      </footer>
    </main>
  );
}
