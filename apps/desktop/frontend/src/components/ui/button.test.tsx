import { fireEvent, render, screen } from "@testing-library/react";
import { createRef } from "react";
import { describe, expect, it, vi } from "vitest";
import { Button } from "./button";

describe("Button composition", () => {
  it("composes an existing link without adding a nested button or losing its ref", () => {
    const onClick = vi.fn();
    const ref = createRef<HTMLButtonElement>();
    render(
      <Button asChild ref={ref} onClick={onClick}>
        <a href="#help">Help</a>
      </Button>,
    );
    const link = screen.getByRole("link", { name: "Help" });
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
    expect(ref.current).toBe(link);
    fireEvent.click(link);
    expect(onClick).toHaveBeenCalledOnce();
  });
});
