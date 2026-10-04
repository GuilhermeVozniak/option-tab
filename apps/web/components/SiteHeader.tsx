import Link from "next/link";
import { ThemeControl } from "./ThemeControl";

export function SiteHeader() {
  return (
    <header className="site-header">
      <div className="site-header-inner">
        <Link href="/" className="brand" aria-label="Option Tab home">
          <img src="/option-tab.svg" width="32" height="32" alt="" />
          <span>Option Tab</span>
        </Link>
        <nav aria-label="Main navigation">
          <Link href="/#features">Features</Link>
          <Link href="/docs">Guide</Link>
          <a href="https://github.com/GuilhermeVozniak/option-tab" className="header-source">
            GitHub
          </a>
        </nav>
        <ThemeControl />
      </div>
    </header>
  );
}
