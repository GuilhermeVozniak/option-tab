const windows = [
  { title: "Project notes", app: "Notes", kind: "notes" },
  { title: "Design review", app: "Browser", kind: "browser", selected: true },
  { title: "option-tab", app: "Terminal", kind: "terminal" },
  { title: "Documents", app: "Finder", kind: "files" },
];

export function WindowGridPreview() {
  return (
    <figure className="window-preview">
      <div className="preview-switcher" aria-hidden="true">
        <div className="preview-toolbar">
          <span>All windows</span>
          <span>
            Close window <kbd>W</kbd>
          </span>
        </div>
        <div className="preview-grid">
          {windows.map((window) => (
            <div key={window.kind} className={`preview-tile${window.selected ? " selected" : ""}`}>
              <div className="preview-title">
                <span className={`preview-app-dot ${window.kind}`} />
                <span>{window.title}</span>
              </div>
              <div className={`preview-content ${window.kind}`}>
                <div className="preview-window-chrome">
                  <i />
                  <i />
                  <i />
                </div>
                <div className="preview-document">
                  <span />
                  <span />
                  <span />
                  <span />
                </div>
                <span className="preview-app-name">{window.app}</span>
              </div>
            </div>
          ))}
        </div>
      </div>
      <figcaption>
        One shortcut. Every window, together. <span>Illustrative preview.</span>
      </figcaption>
    </figure>
  );
}
