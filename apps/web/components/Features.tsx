import Link from "next/link";
import { featureSections } from "@/lib/feature-guide";

const styles = [
  {
    name: "Thumbnails",
    description: "Recognize individual windows at a glance. The default view.",
  },
  { name: "App icons", description: "Switch with a compact row of familiar application icons." },
  { name: "Titles", description: "Keep the list small and focus on window names." },
];

export function Features() {
  return (
    <>
      <section className="content-section styles-section" aria-labelledby="styles-heading">
        <div className="section-heading">
          <h2 id="styles-heading">Three ways to see your windows</h2>
          <p>Choose a visual style for each shortcut. Keep the window grid, or make it your own.</p>
        </div>
        <div className="style-options">
          {styles.map((style) => (
            <div key={style.name}>
              <h3>{style.name}</h3>
              <p>{style.description}</p>
            </div>
          ))}
        </div>
      </section>
      <section className="content-section" id="features" aria-labelledby="features-heading">
        <div className="section-heading">
          <h2 id="features-heading">Everything you can do</h2>
          <p>Start with switching windows. Add the tools that fit the way you work.</p>
        </div>
        <div className="feature-overview">
          {featureSections.map((section) => (
            <Link className="feature-category" href={`/docs#${section.id}`} key={section.id}>
              <h3>{section.title}</h3>
              <p>{section.description}</p>
              <span>
                Explore {section.features.length} topics <span aria-hidden="true">↗</span>
              </span>
            </Link>
          ))}
        </div>
      </section>
    </>
  );
}
