import Link from "next/link";

export function SiteFooter() {
  return (
    <footer className="site-footer">
      <div>
        <strong>Option Tab</strong>
        <p>Free and open source. Made for your Mac.</p>
      </div>
      <nav aria-label="Footer navigation">
        <Link href="/docs">User guide</Link>
        <a href="https://github.com/GuilhermeVozniak/option-tab">Source on GitHub</a>
        <a href="https://github.com/GuilhermeVozniak/option-tab/blob/main/LICENSE">
          Apache-2.0 license
        </a>
      </nav>
    </footer>
  );
}
