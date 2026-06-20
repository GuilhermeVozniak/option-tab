import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { Greeting } from "./Greeting";

describe("Greeting", () => {
  it("renders the provided message", () => {
    render(<Greeting message="Hello, Gui!" />);
    expect(screen.getByRole("status")).toHaveTextContent("Hello, Gui!");
  });
});
