import { fireEvent, render, screen } from "@testing-library/react";
import { expect, it } from "vitest";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "./collapsible";
import { FormDisabledProvider } from "./form-disabled";

it("inherits disabling from its Collapsible root and recovers when enabled", () => {
  const disclosure = (disabled: boolean) => (
    <Collapsible disabled={disabled}>
      <CollapsibleTrigger>Advanced options</CollapsibleTrigger>
      <CollapsibleContent>Advanced content</CollapsibleContent>
    </Collapsible>
  );
  const { rerender } = render(disclosure(true));
  const trigger = screen.getByRole("button", { name: "Advanced options" });
  expect(trigger).toBeDisabled();
  fireEvent.click(trigger);
  expect(trigger).toHaveAttribute("aria-expanded", "false");
  rerender(disclosure(false));
  expect(trigger).toBeEnabled();
  fireEvent.click(trigger);
  expect(screen.getByText("Advanced content")).toBeVisible();
});

it("does not allow an inner form scope to enable an ancestor's disabled controls", () => {
  render(
    <FormDisabledProvider disabled>
      <FormDisabledProvider disabled={false}>
        <Collapsible>
          <CollapsibleTrigger>Advanced options</CollapsibleTrigger>
        </Collapsible>
      </FormDisabledProvider>
    </FormDisabledProvider>,
  );
  const trigger = screen.getByRole("button", { name: "Advanced options" });
  expect(trigger).toBeDisabled();
  fireEvent.click(trigger);
  expect(trigger).toHaveAttribute("aria-expanded", "false");
});
