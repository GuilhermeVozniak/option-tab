"use client";

import { useState } from "react";
import { featureSections } from "@/lib/feature-guide";

export function FeatureGuide() {
  const [query, setQuery] = useState("");
  const normalizedQuery = query.trim().toLowerCase();
  const sections = featureSections
    .map((section) => ({
      ...section,
      features: section.features.filter((feature) =>
        [
          section.title,
          feature.title,
          feature.description,
          ...feature.howTo,
          ...(feature.notes ?? []),
        ]
          .join(" ")
          .toLowerCase()
          .includes(normalizedQuery),
      ),
    }))
    .filter((section) => section.features.length > 0);
  const count = sections.reduce((total, section) => total + section.features.length, 0);

  const navigation = (
    <nav aria-label="Guide sections">
      {sections.map((section) => (
        <a href={`#${section.id}`} key={section.id}>
          {section.title}
        </a>
      ))}
    </nav>
  );

  return (
    <div className="guide-layout">
      <aside className="guide-sidebar">
        <span className="guide-nav-title">In this guide</span>
        {navigation}
      </aside>
      <div className="guide-content">
        <div className="guide-search">
          <label htmlFor="guide-search">Find a feature or setting</label>
          <div className="guide-search-input">
            <input
              id="guide-search"
              type="search"
              placeholder="Try “Dock previews” or “shortcuts”"
              value={query}
              onChange={(event) => setQuery(event.target.value)}
            />
            {query && (
              <button type="button" onClick={() => setQuery("")}>
                Clear
              </button>
            )}
          </div>
          <p role="status">
            {normalizedQuery
              ? `${count} matching ${count === 1 ? "topic" : "topics"}`
              : `${count} topics covering every Settings section`}
          </p>
        </div>
        <details className="guide-mobile-nav">
          <summary>Browse guide sections</summary>
          {navigation}
        </details>
        {sections.length === 0 && (
          <div className="guide-empty">
            <h2>No matching topics</h2>
            <p>Try a feature name such as “windows”, “media”, or “permissions”.</p>
          </div>
        )}
        {sections.map((section) => (
          <section id={section.id} className="guide-section" key={section.id}>
            <div className="guide-section-heading">
              <h2>
                <a href={`#${section.id}`}>{section.title}</a>
              </h2>
              <p>{section.description}</p>
            </div>
            {section.features.map((feature) => (
              <article className="guide-feature" id={feature.id} key={feature.id}>
                <h3>
                  <a href={`#${feature.id}`}>{feature.title}</a>
                </h3>
                <p>{feature.description}</p>
                <ol>
                  {feature.howTo.map((step) => (
                    <li key={step}>{step}</li>
                  ))}
                </ol>
                {feature.notes && (
                  <div className="guide-notes">
                    {feature.notes.map((note) => (
                      <p key={note}>{note}</p>
                    ))}
                  </div>
                )}
              </article>
            ))}
          </section>
        ))}
      </div>
    </div>
  );
}
