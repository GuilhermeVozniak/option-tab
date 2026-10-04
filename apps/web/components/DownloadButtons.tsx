import { publishedDownloadUrl } from "../lib/download";

export function DownloadButtons() {
  return (
    <div className="download-options">
      <a data-testid="download-darwin" href={publishedDownloadUrl("darwin")}>
        macOS download · Apple silicon &amp; Intel
      </a>
      <details>
        <summary>Other platforms</summary>
        <p>Windows and Linux demo builds; native window switching is macOS-only.</p>
        <nav aria-label="Demo downloads">
          <a data-testid="download-windows" href={publishedDownloadUrl("windows")}>
            Windows (demo)
          </a>
          <a data-testid="download-linux" href={publishedDownloadUrl("linux")}>
            Linux (demo)
          </a>
        </nav>
      </details>
    </div>
  );
}
