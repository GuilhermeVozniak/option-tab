import type { Metadata } from "next";
import { FeatureGuide } from "../../components/FeatureGuide";
import { featureGuideIntro } from "../../lib/feature-guide";

export const metadata: Metadata = {
  title: "User guide — Option Tab",
  description:
    "Learn how to use Option Tab: window and app switching, keyboard shortcuts, Dock previews, media controls, widgets, automation, and settings.",
  alternates: { canonical: "/docs/" },
  openGraph: {
    title: "User guide — Option Tab",
    url: "/docs/",
    description: "Every Option Tab feature, with setup instructions and the exact settings to use.",
  },
};

export default function DocsPage() {
  return (
    <main id="main-content" className="site-main docs-main">
      <header className="docs-heading">
        <h1>User guide</h1>
        <p className="docs-lead">Get comfortable with Option Tab.</p>
        {featureGuideIntro.map((paragraph) => (
          <p key={paragraph}>{paragraph}</p>
        ))}
      </header>
      <FeatureGuide />
      <section className="guide-references" aria-labelledby="references-heading">
        <h2 id="references-heading">Further reading</h2>
        <p>Detailed reference material for scripting, local data, and installation.</p>
        <nav aria-label="Reference guides">
          <a href="https://github.com/GuilhermeVozniak/option-tab/blob/main/docs/automation.md">
            AppleScript commands and examples
          </a>
          <a href="https://github.com/GuilhermeVozniak/option-tab/blob/main/docs/data-handling.md">
            Data handling and permissions
          </a>
          <a href="https://github.com/GuilhermeVozniak/option-tab/blob/main/docs/homebrew.md">
            Homebrew installation and updates
          </a>
        </nav>
      </section>
    </main>
  );
}
