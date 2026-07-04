import { downloadUrl } from "@option-tab/shared";
import { APP_VERSION } from "../lib/download";

const TARGETS = [
  { label: "Download for macOS", platform: "darwin", arch: "arm64" },
  { label: "Download for Windows", platform: "windows", arch: "amd64" },
  { label: "Download for Linux", platform: "linux", arch: "amd64" },
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
              href={downloadUrl(t.platform, t.arch, APP_VERSION)}
            >
              {t.label}
            </a>
          </li>
        ))}
      </ul>
    </nav>
  );
}
