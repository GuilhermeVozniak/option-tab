import { publishedDownloadUrl } from "../lib/download";

const TARGETS = [
  { label: "Download for macOS (Apple silicon)", platform: "darwin" },
  { label: "Download for Windows", platform: "windows" },
  { label: "Download for Linux", platform: "linux" },
] as const;

export function DownloadButtons() {
  return (
    <nav aria-label="Downloads">
      <ul className="m-0 mb-4 flex list-none flex-wrap justify-center gap-5 p-0">
        {TARGETS.map((t) => (
          <li key={t.platform}>
            <a
              data-testid={`download-${t.platform}`}
              className="text-[15px] text-muted-foreground no-underline transition-colors hover:text-foreground hover:underline"
              href={publishedDownloadUrl(t.platform)}
            >
              {t.label}
            </a>
          </li>
        ))}
      </ul>
    </nav>
  );
}
