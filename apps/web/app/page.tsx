import Link from "next/link";
import { DownloadButtons } from "../components/DownloadButtons";
import { Features } from "../components/Features";
import { PrimaryDownload } from "../components/PrimaryDownload";
import { buttonVariants } from "../components/ui/button";
import { WindowGridPreview } from "../components/WindowGridPreview";
import { latestReleaseUrl } from "../lib/download";

export default function Home() {
  return (
    <main id="main-content" className="site-main">
      <section className="hero">
        <div className="hero-copy">
          <div className="hero-product">
            <img src="/option-tab.svg" width="56" height="56" alt="" />
            <span>Free &amp; open source for macOS</span>
          </div>
          <h1>Option Tab</h1>
          <p className="hero-lead">
            Find the window.
            <br />
            Get back to what matters.
          </p>
          <p className="hero-description">
            See your open windows, pick the one you need, and keep moving. A window switcher, Dock
            companion, and keyboard toolkit that works your way.
          </p>
          <div className="hero-actions">
            <PrimaryDownload />
            <Link className={buttonVariants({ variant: "glass", size: "lg" })} href="/docs">
              Read the guide
            </Link>
          </div>
          <p className="system-requirements">macOS 14+ · Apple silicon &amp; Intel · no account</p>
          <DownloadButtons />
        </div>
        <WindowGridPreview />
      </section>
      <section className="quick-start" aria-labelledby="quick-start-heading">
        <div>
          <h2 id="quick-start-heading">Your windows, by default.</h2>
          <p>
            On a fresh install, both <kbd>⌘</kbd> + <kbd>Tab</kbd> and <kbd>⌥</kbd> + <kbd>Tab</kbd>{" "}
            open the thumbnail grid of all windows. There is no separate enlarged preview.
          </p>
        </div>
        <Link href="/docs#getting-started">
          Set up Option Tab <span aria-hidden="true">↗</span>
        </Link>
      </section>
      <Features />
      <section className="content-section install-section" aria-labelledby="install-heading">
        <div className="section-heading">
          <h2 id="install-heading">Install it your way</h2>
          <p>Download the app, or install through the personal Homebrew tap.</p>
        </div>
        <div className="install-command">
          <span>Homebrew</span>
          <code>brew install --cask GuilhermeVozniak/tap/option-tab</code>
        </div>
        <p className="install-links">
          <Link href="/docs#getting-started">Installation and permissions</Link>
          <a href={latestReleaseUrl()}>All releases</a>
        </p>
      </section>
    </main>
  );
}
