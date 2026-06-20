import { DownloadButtons } from "../components/DownloadButtons";
import { PrimaryDownload } from "../components/PrimaryDownload";

export default function Home() {
  return (
    <main>
      <h1>Option Tab</h1>
      <PrimaryDownload />
      <p>A fast, open-source desktop app. Free forever.</p>
      <DownloadButtons />
    </main>
  );
}
