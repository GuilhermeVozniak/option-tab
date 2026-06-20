import { DownloadButtons } from "../components/DownloadButtons";
import { Features } from "../components/Features";
import { PrimaryDownload } from "../components/PrimaryDownload";
import { latestReleaseUrl } from "../lib/download";

export default function Home() {
  return (
    <main>
      <section className="hero">
        <span className="badge">100% free &amp; open-source — every Pro feature included</span>
        <h1>Option Tab</h1>
        <p className="tagline">
          The Windows-style <kbd>Alt</kbd>+<kbd>Tab</kbd> window switcher for macOS — switch by
          window, not just by app, with live previews, fuzzy search, and up to 9 custom shortcuts.
        </p>
        <div className="cta">
          <PrimaryDownload />
          <a className="ghost" href={latestReleaseUrl()}>
            All releases
          </a>
        </div>
        <DownloadButtons />
        <p className="subnote">
          macOS 13+ · Windows &amp; Linux builds available · no account, no paywall
        </p>
      </section>

      <Features />

      <footer className="footer">
        <p>
          Free forever under the Apache-2.0 license.{" "}
          <a href="https://github.com/GuilhermeVozniak/option-tab">Source on GitHub</a>.
        </p>
        <p className="credit">
          Inspired by AltTab — rebuilt in Go &amp; Wails, with the paid features free for everyone.
        </p>
      </footer>
    </main>
  );
}
