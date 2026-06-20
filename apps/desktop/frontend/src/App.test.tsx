import { render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import App from "./App";

afterEach(() => {
  window.location.hash = "";
});

describe("App", () => {
  it("renders the overlay route (closed) by default", () => {
    window.location.hash = "";
    const { container } = render(<App />);
    // Overlay renders nothing while closed.
    expect(container.firstChild).toBeNull();
  });

  it("renders the settings route at #settings", () => {
    window.location.hash = "#settings";
    render(<App />);
    expect(screen.getByText(/Preferences/)).toBeInTheDocument();
  });
});
